package main

import (
	"context"

	"golang.design/x/clipboard"
)

func initClipboard() error {
	return clipboard.Init()
}

func watchText(ctx context.Context) <-chan []byte {
	return clipboard.Watch(ctx, clipboard.FmtText)
}

func writeText(data []byte) {
	clipboard.Write(clipboard.FmtText, data)
}

func readText() []byte {
	return clipboard.Read(clipboard.FmtText)
}

func watchImage(ctx context.Context) <-chan []byte {
	return clipboard.Watch(ctx, clipboard.FmtImage)
}

func writeImage(data []byte) {
	clipboard.Write(clipboard.FmtImage, data)
}

func readImage() []byte {
	return clipboard.Read(clipboard.FmtImage)
}
