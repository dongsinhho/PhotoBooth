//go:build !windows

package output

import "errors"

func InspectWindowsPrinterJob(queueName, documentName, jobID string) (PrinterJobInspection, error) {
	return PrinterJobInspection{}, errors.New("Windows printer queue inspection is supported only on Windows")
}
