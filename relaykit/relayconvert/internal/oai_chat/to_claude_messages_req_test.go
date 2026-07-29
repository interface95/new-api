package oaichat

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	relaymedia "github.com/QuantumNous/new-api/relaykit/relayconvert/internal/media"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIChatRequestToClaudeMessagesPreservesTextFile(t *testing.T) {
	relaymedia.SetMediaResolver(relaymedia.MediaResolver{
		GetBase64Data: func(_ context.Context, source types.FileSource, _ ...string) (string, string, error) {
			base64Source, ok := source.(*types.Base64Source)
			require.True(t, ok)
			assert.Contains(t, base64Source.MimeType, "text/plain")
			return "aGVsbG8=", base64Source.MimeType, nil
		},
	})
	t.Cleanup(func() {
		relaymedia.SetMediaResolver(relaymedia.MediaResolver{})
	})

	maxTokens := uint(128)
	message := dto.Message{
		Role: "user",
		Content: []any{
			map[string]any{
				"type": dto.ContentTypeFile,
				"file": map[string]any{
					"filename":  "note.txt",
					"file_data": "aGVsbG8=",
				},
			},
		},
	}

	request, err := OpenAIChatRequestToClaudeMessages(context.Background(), &convmeta.Values{}, dto.GeneralOpenAIRequest{
		Model:     "claude-test",
		MaxTokens: &maxTokens,
		Messages:  []dto.Message{message},
	})

	require.NoError(t, err)
	require.Len(t, request.Messages, 1)
	content, ok := request.Messages[0].Content.([]dto.ClaudeMediaMessage)
	require.True(t, ok)
	require.Len(t, content, 1)
	assert.Equal(t, "text", content[0].Type)
	require.NotNil(t, content[0].Text)
	assert.Equal(t, "hello", *content[0].Text)
	assert.Nil(t, content[0].Source)
}
