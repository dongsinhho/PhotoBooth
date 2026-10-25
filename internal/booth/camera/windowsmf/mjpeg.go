package windowsmf

import (
	"encoding/hex"
	"errors"
	"fmt"
)

const defaultHuffmanTablesHex = "" +
	"ffc4001f0000010501010101010100000000000000000102030405060708090a0b" +
	"ffc400b5100002010303020403050504040000017d01020300041105122131410613516107227114328191a1082342b1c11552d1f02433627282090a161718191a25262728292a3435363738393a434445464748494a535455565758595a636465666768696a737475767778797a838485868788898a92939495969798999aa2a3a4a5a6a7a8a9aab2b3b4b5b6b7b8b9bac2c3c4c5c6c7c8c9cad2d3d4d5d6d7d8d9dae1e2e3e4e5e6e7e8e9eaf1f2f3f4f5f6f7f8f9fa" +
	"ffc4001f0100030101010101010101010000000000000102030405060708090a0b" +
	"ffc400b51100020102040403040705040400010277000102031104052131061241510761711322328108144291a1b1c109233352f0156272d10a162434e125f11718191a262728292a35363738393a434445464748494a535455565758595a636465666768696a737475767778797a82838485868788898a92939495969798999aa2a3a4a5a6a7a8a9aab2b3b4b5b6b7b8b9bac2c3c4c5c6c7c8c9cad2d3d4d5d6d7d8d9dae2e3e4e5e6e7e8e9eaf2f3f4f5f6f7f8f9fa"

// normalizeMJPEG inserts the standard JPEG Huffman tables when a webcam emits
// an abbreviated MJPEG frame that omits DHT markers. OpenCV accepts these
// frames directly; Go's image/jpeg decoder correctly rejects them without DHT.
// The frame is never decoded before this structural check and repair.
func normalizeMJPEG(data []byte) ([]byte, error) {
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return nil, errors.New("invalid JPEG start marker")
	}
	i := 2
	hasDHT := false
	hasSOS := false
	for i < len(data) {
		if data[i] != 0xff {
			return nil, fmt.Errorf("invalid JPEG marker prefix at byte %d", i)
		}
		for i < len(data) && data[i] == 0xff {
			i++
		}
		if i >= len(data) {
			return nil, errors.New("truncated JPEG marker")
		}
		marker := data[i]
		i++
		if marker == 0x00 {
			return nil, errors.New("unexpected stuffed byte before JPEG scan")
		}
		if marker == 0xda {
			hasSOS = true
			break
		}
		if marker == 0xd8 || marker == 0xd9 || marker == 0x01 || marker >= 0xd0 && marker <= 0xd7 {
			continue
		}
		if i+2 > len(data) {
			return nil, errors.New("truncated JPEG segment length")
		}
		segmentLength := int(data[i])<<8 | int(data[i+1])
		if segmentLength < 2 || i+segmentLength > len(data) {
			return nil, errors.New("invalid JPEG segment length")
		}
		if marker == 0xc4 {
			hasDHT = true
		}
		i += segmentLength
	}
	if !hasSOS {
		return nil, errors.New("JPEG scan marker not found")
	}
	if hasDHT {
		return data, nil
	}
	tables, err := hex.DecodeString(defaultHuffmanTablesHex)
	if err != nil {
		return nil, fmt.Errorf("decode built-in JPEG Huffman tables: %w", err)
	}
	out := make([]byte, 0, len(data)+len(tables))
	out = append(out, data[:2]...)
	out = append(out, tables...)
	out = append(out, data[2:]...)
	return out, nil
}
