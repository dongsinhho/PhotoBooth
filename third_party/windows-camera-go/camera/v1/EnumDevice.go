package camera

import (
	"errors"
	"fmt"

	"github.com/Kirizu-Official/windows-camera-go/utils"
	"github.com/Kirizu-Official/windows-camera-go/windows/guid"
	"github.com/Kirizu-Official/windows-camera-go/windows/mf"
)

type DeviceInfo struct {
	Name       string `json:"name" yaml:"name"`
	SymbolLink string `json:"symbol_link" yaml:"symbol_link"`
}

type CaptureFormats struct {
	DescriptorIndex    uint32     `json:"descriptor_index" yaml:"descriptor_index"`
	MediaTypeIndex     uint32     `json:"media_type_index" yaml:"media_type_index"`
	IsCompressedFormat bool       `json:"is_compressed_format" yaml:"is_compressed_format"`
	MajorType          *guid.GUID `json:"major_type" yaml:"major_type"`
	SubType            *guid.GUID `json:"sub_type" yaml:"sub_type"`
	Width              uint32     `json:"width" yaml:"width"`
	Height             uint32     `json:"height" yaml:"height"`
	Fps                float64    `json:"fps" yaml:"fps"`
}

func EnumDevice() ([]*DeviceInfo, error) {
	list, err := callEnumDeviceSources()
	if err != nil {
		return nil, err
	}
	var deviceList []*DeviceInfo
	var metadataErrors []error
	for i, source := range list {
		info, infoErr := callGetBaseInfo(source)
		if infoErr != nil {
			metadataErrors = append(metadataErrors, fmt.Errorf("camera source %d: %w", i, infoErr))
			_ = source.Release()
			continue
		}
		deviceList = append(deviceList, info)
		_ = source.Release()
	}
	if len(deviceList) == 0 {
		if err := allCameraSourceMetadataFailed(len(list), metadataErrors); err != nil {
			return nil, err
		}
	}
	return deviceList, nil
}

// allCameraSourceMetadataFailed distinguishes an empty native enumeration
// from activation sources that exist but cannot be converted into camera
// metadata. The latter must not silently look like "no camera devices".
func allCameraSourceMetadataFailed(sourceCount int, metadataErrors []error) error {
	if sourceCount == 0 || len(metadataErrors) == 0 {
		return nil
	}
	return fmt.Errorf(
		"Media Foundation returned %d camera source(s), but metadata lookup failed for all: %w",
		sourceCount,
		errors.Join(metadataErrors...),
	)
}

func (d *Device) EnumerateCaptureFormats() ([]*CaptureFormats, error) {
	decCount, err := d.pDevPresent.GetStreamDescriptorCount()
	if err != nil {
		return nil, err
	}
	var result []*CaptureFormats
	var typeCount uint32

	for index := 0; index < int(decCount); index++ {
		descriptor, _, err := d.getStreamDescriptorByIndex(uint32(index))
		if err != nil {
			fmt.Println("[WARN]GetStreamDescriptorByIndex: " + err.Error())
			continue
		}
		var MediaTypeHandler *mf.IMFMediaTypeHandler
		err = descriptor.GetMediaTypeHandler(&MediaTypeHandler)
		if err != nil {
			fmt.Println("[WARN]GetMediaTypeHandler: " + err.Error())
			descriptor.Release()
			continue
		}

		typeCount, err = MediaTypeHandler.GetMediaTypeCount()
		for n := 0; n < int(typeCount); n++ {
			var mediaType *mf.IMFMediaType
			err = MediaTypeHandler.GetMediaTypeByIndex(uint32(n), &mediaType)
			if err != nil {
				fmt.Println("[WARN]GetMediaTypeByIndex: " + err.Error())
				mediaType.Release()
				continue
			}
			info, err := d.getMediaTypeInfo(mediaType)
			if err != nil {
				fmt.Println("[WARN]GetMediaTypeInfo: " + err.Error())
				mediaType.Release()
				continue
			}
			info.DescriptorIndex = uint32(index)
			info.MediaTypeIndex = uint32(n)
			result = append(result, info)
			mediaType.Release()
		}
		MediaTypeHandler.Release()
		descriptor.Release()

	}
	return result, nil
}
func (d *Device) getSelectedMediaType(format *CaptureFormats) (*mf.IMFMediaType, error) {

	descriptor, _, err := d.getStreamDescriptorByIndex(format.DescriptorIndex)
	if err != nil {
		return nil, err
	}

	var MediaTypeHandler *mf.IMFMediaTypeHandler
	err = descriptor.GetMediaTypeHandler(&MediaTypeHandler)
	defer descriptor.Release()
	if err != nil {
		return nil, err
	}
	defer MediaTypeHandler.Release()

	var mediaType *mf.IMFMediaType
	err = MediaTypeHandler.GetMediaTypeByIndex(format.MediaTypeIndex, &mediaType)
	if err != nil {
		return nil, err
	}

	info, err := d.getMediaTypeInfo(mediaType)
	if err != nil {
		mediaType.Release()
		return nil, err
	}

	if info.IsCompressedFormat != format.IsCompressedFormat || !(*guid.GUID)(info.MajorType).IsMatch(format.MajorType) || !(*guid.GUID)(info.SubType).IsMatch(format.SubType) || info.Height != format.Height || info.Width != format.Width || info.Fps != format.Fps {
		mediaType.Release()
		return nil, utils.ErrorFormatNotMatched
	}

	return mediaType, nil
}

func (d *Device) getStreamDescriptorByIndex(index uint32) (*mf.IMFStreamDescriptor, bool, error) {
	var descriptor *mf.IMFStreamDescriptor
	var isSelected bool
	err := d.pDevPresent.GetStreamDescriptorByIndex(index, &isSelected, &descriptor)
	return descriptor, isSelected, err
}
func (d *Device) getMediaTypeInfo(mediaType *mf.IMFMediaType) (*CaptureFormats, error) {
	isCompressed, err := mediaType.IsCompressedFormat()
	if err != nil {
		return nil, err
	}
	var majorType guid.GUID
	err = mediaType.GetMajorType(&majorType)
	if err != nil {
		return nil, err
	}

	var subType guid.GUID
	err = mediaType.GetGUID(guid.MF_MT_SUBTYPE, &subType)
	if err != nil {
		return nil, err
	}

	width, height, err := mediaType.GetFrameSize()
	if err != nil {
		return nil, err
	}
	fps, err := mediaType.GetFrameRate()
	if err != nil {
		return nil, err
	}
	return &CaptureFormats{
		IsCompressedFormat: isCompressed,
		MajorType:          &majorType,
		SubType:            &subType,
		Width:              width,
		Height:             height,
		Fps:                fps,
	}, nil
}
