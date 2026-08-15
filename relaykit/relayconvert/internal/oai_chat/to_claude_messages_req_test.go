package oaichat

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIChatRequestToClaudeMessagesConvertsTextFileToTextBlock(t *testing.T) {
	maxTokens := uint(128)
	message := dto.Message{
		Role: "user",
		Content: []any{
			map[string]any{
				"type": dto.ContentTypeFile,
				"file": map[string]any{
					"filename":  "notes.md",
					"file_data": base64.StdEncoding.EncodeToString([]byte("release notes")),
				},
			},
		},
	}

	got, err := OpenAIChatRequestToClaudeMessages(context.Background(), nil, dto.GeneralOpenAIRequest{
		Model:     "claude-test",
		MaxTokens: &maxTokens,
		Messages:  []dto.Message{message},
	})
	require.NoError(t, err)
	require.Len(t, got.Messages, 1)

	content, ok := got.Messages[0].Content.([]dto.ClaudeMediaMessage)
	require.True(t, ok)
	require.Len(t, content, 1)
	assert.Equal(t, "text", content[0].Type)
	assert.Equal(t, "release notes", content[0].GetText())
}
