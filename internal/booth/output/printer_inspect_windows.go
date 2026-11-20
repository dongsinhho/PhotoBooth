//go:build windows

package output

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	maxPrinterQueueJobs   = 4096
	maxPrinterQueueBuffer = 4 << 20

	jobStatusPaused           = 0x00000001
	jobStatusError            = 0x00000002
	jobStatusDeleting         = 0x00000004
	jobStatusSpooling         = 0x00000008
	jobStatusPrinting         = 0x00000010
	jobStatusOffline          = 0x00000020
	jobStatusPaperOut         = 0x00000040
	jobStatusPrinted          = 0x00000080
	jobStatusDeleted          = 0x00000100
	jobStatusBlockedDevQueue  = 0x00000200
	jobStatusUserIntervention = 0x00000400
	jobStatusRestart          = 0x00000800
	jobStatusComplete         = 0x00001000
	jobStatusRetained         = 0x00002000
	jobStatusRenderingLocally = 0x00004000
)

var enumJobsWProc = syscall.NewLazyDLL("winspool.drv").NewProc("EnumJobsW")

type jobInfo1W struct {
	jobID        uint32
	printerName  *uint16
	machineName  *uint16
	userName     *uint16
	documentName *uint16
	datatype     *uint16
	statusText   *uint16
	status       uint32
	priority     uint32
	position     uint32
	startTime    uint32
	untilTime    uint32
	totalPages   uint32
	pagesPrinted uint32
}

// InspectWindowsPrinterJob enumerates a bounded snapshot of the selected local
// queue and returns only jobs matching the exact persisted job ID and/or
// document name. It does not pause, resume, cancel, delete or resubmit jobs.
func InspectWindowsPrinterJob(queueName, documentName, jobID string) (PrinterJobInspection, error) {
	queueName = strings.TrimSpace(queueName)
	documentName = strings.TrimSpace(documentName)
	jobID = strings.TrimSpace(jobID)
	if queueName == "" || len(queueName) > 256 || strings.ContainsAny(queueName, "\r\n\x00") {
		return PrinterJobInspection{}, errors.New("printer queue name is invalid")
	}
	if documentName == "" && jobID == "" {
		return PrinterJobInspection{}, errors.New("printer inspection requires a persisted document name or job ID")
	}
	var numericJobID uint64
	var err error
	if jobID != "" {
		numericJobID, err = strconv.ParseUint(jobID, 10, 32)
		if err != nil || numericJobID == 0 {
			return PrinterJobInspection{}, fmt.Errorf("persisted Windows job ID %q is invalid", jobID)
		}
	}

	queueUTF16, err := syscall.UTF16PtrFromString(queueName)
	if err != nil {
		return PrinterJobInspection{}, fmt.Errorf("encode printer queue name: %w", err)
	}
	var printerHandle syscall.Handle
	opened, _, callErr := openPrinterWProc.Call(
		uintptr(unsafe.Pointer(queueUTF16)),
		uintptr(unsafe.Pointer(&printerHandle)),
		0,
	)
	if opened == 0 {
		return PrinterJobInspection{}, win32CallError("OpenPrinterW for queue inspection", callErr)
	}
	defer closePrinterProc.Call(uintptr(printerHandle))

	var needed uint32
	var returned uint32
	enumerated, _, callErr := enumJobsWProc.Call(
		uintptr(printerHandle),
		0,
		maxPrinterQueueJobs,
		1,
		0,
		0,
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&returned)),
	)
	if enumerated == 0 && callErr != syscall.Errno(122) { // ERROR_INSUFFICIENT_BUFFER is the normal sizing response.
		return PrinterJobInspection{}, win32CallError("EnumJobsW(size)", callErr)
	}
	inspection := PrinterJobInspection{QueueName: queueName, Jobs: []PrinterQueueJob{}}
	if needed == 0 || returned == 0 {
		return inspection, nil
	}
	if needed > maxPrinterQueueBuffer {
		return PrinterJobInspection{}, fmt.Errorf("printer queue snapshot exceeds the %d-byte safety limit", maxPrinterQueueBuffer)
	}
	buffer := make([]byte, int(needed))
	needed = 0
	returned = 0
	enumerated, _, callErr = enumJobsWProc.Call(
		uintptr(printerHandle),
		0,
		maxPrinterQueueJobs,
		1,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&returned)),
	)
	if enumerated == 0 {
		return PrinterJobInspection{}, win32CallError("EnumJobsW(data)", callErr)
	}
	if returned > maxPrinterQueueJobs {
		return PrinterJobInspection{}, errors.New("Windows returned an invalid printer job count")
	}
	stride := unsafe.Sizeof(jobInfo1W{})
	for index := uint32(0); index < returned; index++ {
		offset := uintptr(index) * stride
		if offset+stride > uintptr(len(buffer)) {
			return PrinterJobInspection{}, errors.New("Windows returned a truncated printer job snapshot")
		}
		job := (*jobInfo1W)(unsafe.Add(unsafe.Pointer(&buffer[0]), offset))
		if jobID != "" && uint64(job.jobID) != numericJobID {
			continue
		}
		currentDocument := utf16PtrToString(job.documentName)
		if documentName != "" && currentDocument != documentName {
			continue
		}
		inspection.Jobs = append(inspection.Jobs, PrinterQueueJob{
			JobID:        strconv.FormatUint(uint64(job.jobID), 10),
			DocumentName: currentDocument,
			DriverStatus: utf16PtrToString(job.statusText),
			StatusFlags:  job.status,
			Status:       describePrinterJobStatus(job.status),
			Position:     job.position,
			TotalPages:   job.totalPages,
			PagesPrinted: job.pagesPrinted,
		})
	}
	inspection.Found = len(inspection.Jobs) > 0
	return inspection, nil
}

func utf16PtrToString(value *uint16) string {
	if value == nil {
		return ""
	}
	const maxUTF16Units = 32768
	units := make([]uint16, 0, 128)
	base := unsafe.Pointer(value)
	for index := 0; index < maxUTF16Units; index++ {
		current := *(*uint16)(unsafe.Add(base, uintptr(index)*unsafe.Sizeof(*value)))
		if current == 0 {
			return syscall.UTF16ToString(units)
		}
		units = append(units, current)
	}
	return syscall.UTF16ToString(units)
}

func describePrinterJobStatus(flags uint32) []string {
	type namedFlag struct {
		mask uint32
		name string
	}
	all := []namedFlag{
		{jobStatusPaused, "paused"},
		{jobStatusError, "error"},
		{jobStatusDeleting, "deleting"},
		{jobStatusSpooling, "spooling"},
		{jobStatusPrinting, "printing"},
		{jobStatusOffline, "offline"},
		{jobStatusPaperOut, "paper_out"},
		{jobStatusPrinted, "printed"},
		{jobStatusDeleted, "deleted"},
		{jobStatusBlockedDevQueue, "blocked_device_queue"},
		{jobStatusUserIntervention, "user_intervention"},
		{jobStatusRestart, "restart"},
		{jobStatusComplete, "complete"},
		{jobStatusRetained, "retained"},
		{jobStatusRenderingLocally, "rendering_locally"},
	}
	var statuses []string
	for _, flag := range all {
		if flags&flag.mask != 0 {
			statuses = append(statuses, flag.name)
		}
	}
	if len(statuses) == 0 {
		statuses = append(statuses, "queued_or_driver_unspecified")
	}
	return statuses
}
