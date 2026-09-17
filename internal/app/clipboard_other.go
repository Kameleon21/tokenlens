//go:build !darwin && !linux

package app

import (
	"context"

	"github.com/atotto/clipboard"
)

func writeClipboard(ctx context.Context, text string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return clipboard.WriteAll(text)
}
