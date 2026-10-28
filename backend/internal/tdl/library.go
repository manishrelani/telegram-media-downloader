package tdl

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"example.com/telegram-media-downloader/backend/internal/domain"
)

func BuildMediaExportArgs(query domain.MediaQuery, output string) ([]string, error) {
	if query.Chat.ID == 0 || strings.TrimSpace(output) == "" || query.Limit < 0 {
		return nil, fmt.Errorf("invalid media query")
	}
	args := []string{"chat", "export", "-c", strconv.FormatInt(query.Chat.ID, 10), "--with-content", "--raw", "-o", output}
	if query.Topic != nil {
		if query.Topic.ID <= 0 {
			return nil, fmt.Errorf("topic ID must be positive")
		}
		args = append(args, "--topic", strconv.FormatInt(query.Topic.ID, 10))
	}
	if query.Limit > 0 {
		args = append(args, "-T", "last", "-i", strconv.Itoa(query.Limit))
	}
	return args, nil
}

func (a DomainAdapter) ExportMedia(ctx context.Context, query domain.MediaQuery, output string) error {
	args, err := BuildMediaExportArgs(query, output)
	if err != nil {
		return err
	}
	e, err := a.Client.executor()
	if err != nil {
		return err
	}
	_, stderr, err := e.Run(ctx, args...)
	if err != nil {
		return commandError("tdl media export", stderr, err)
	}
	return nil
}
