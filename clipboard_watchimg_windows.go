//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"image/png"
	"log"
	"syscall"
	"time"
	"unsafe"

	"golang.design/x/clipboard"
	"golang.org/x/image/bmp"
)

var (
	user32w   = syscall.NewLazyDLL("user32.dll")
	kernel32w = syscall.NewLazyDLL("kernel32.dll")

	procOpenClipboard              = user32w.NewProc("OpenClipboard")
	procCloseClipboard             = user32w.NewProc("CloseClipboard")
	procGetClipboardData           = user32w.NewProc("GetClipboardData")
	procIsClipboardFormatAvailable = user32w.NewProc("IsClipboardFormatAvailable")
	procGetClipboardSequenceNumber = user32w.NewProc("GetClipboardSequenceNumber")
	procGlobalLock                 = kernel32w.NewProc("GlobalLock")
	procGlobalUnlock               = kernel32w.NewProc("GlobalUnlock")
	procGlobalSize                 = kernel32w.NewProc("GlobalSize")
)

const cfDIB = 8

// readImageDIB reads the clipboard image as CF_DIB and converts to PNG.
// This handles cases where the library's Read(FmtImage) fails, e.g. when
// Windows screenshots produce 24-bit DIBV5 data that the library rejects.
func readImageDIB() []byte {
	r, _, _ := procOpenClipboard.Call(0)
	if r == 0 {
		return nil
	}
	defer procCloseClipboard.Call()

	r, _, _ = procIsClipboardFormatAvailable.Call(cfDIB)
	if r == 0 {
		return nil
	}

	hMem, _, _ := procGetClipboardData.Call(cfDIB)
	if hMem == 0 {
		return nil
	}

	ptrVal, _, _ := procGlobalLock.Call(hMem)
	if ptrVal == 0 {
		return nil
	}
	defer procGlobalUnlock.Call(hMem)

	size, _, _ := procGlobalSize.Call(hMem)
	if size == 0 {
		return nil
	}

	// Copy DIB data from global memory.
	// nolint: the uintptr→unsafe.Pointer conversion is safe here because
	// ptrVal comes from GlobalLock and is valid until GlobalUnlock.
	dibData := make([]byte, size)
	copy(dibData, unsafe.Slice((*byte)(*(*unsafe.Pointer)(unsafe.Pointer(&ptrVal))), size))

	if len(dibData) < 40 {
		return nil
	}
	headerSize := binary.LittleEndian.Uint32(dibData[:4])

	// Compute pixel data offset: header + optional color table.
	dataOffset := headerSize
	bitCount := binary.LittleEndian.Uint16(dibData[14:16])
	compression := binary.LittleEndian.Uint32(dibData[16:20])
	if bitCount <= 8 {
		clrUsed := binary.LittleEndian.Uint32(dibData[32:36])
		if clrUsed == 0 {
			clrUsed = 1 << bitCount
		}
		dataOffset += clrUsed * 4
	} else if headerSize == 40 && compression == 3 /* BI_BITFIELDS */ {
		dataOffset += 12 // three DWORD color masks
	}

	// Construct BMP file: 14-byte file header + DIB data.
	fileHeader := make([]byte, 14)
	fileHeader[0] = 'B'
	fileHeader[1] = 'M'
	binary.LittleEndian.PutUint32(fileHeader[2:6], uint32(14+len(dibData)))
	binary.LittleEndian.PutUint32(fileHeader[10:14], 14+dataOffset)

	bmpData := append(fileHeader, dibData...)

	img, err := bmp.Decode(bytes.NewReader(bmpData))
	if err != nil {
		log.Printf("[clipboard] DIB fallback: bmp decode failed: %v", err)
		return nil
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Printf("[clipboard] DIB fallback: png encode failed: %v", err)
		return nil
	}

	return buf.Bytes()
}

// watchImage watches the clipboard for image changes on Windows.
// It wraps the library's Read with a CF_DIB fallback for screenshots
// that the library cannot read (e.g. 24-bit DIBV5 from Win+Shift+S).
func watchImage(ctx context.Context) <-chan []byte {
	ch := make(chan []byte, 1)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		seq, _, _ := procGetClipboardSequenceNumber.Call()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cur, _, _ := procGetClipboardSequenceNumber.Call()
				if cur == seq {
					continue
				}
				seq = cur
				// Try the library's read first (handles 32-bit DIBV5).
				data := clipboard.Read(clipboard.FmtImage)
				if data == nil {
					// Library failed; try CF_DIB fallback (handles all bit depths).
					data = readImageDIB()
					if data != nil {
						log.Printf("[clipboard] image read via CF_DIB fallback (%d bytes)", len(data))
					}
				}
				if len(data) == 0 {
					continue
				}
				select {
				case ch <- data:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return ch
}
