package cli

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newSessionAttachmentsCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachments",
		Short: "Manage session attachments",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newSessionAttachmentsUploadCommand(deps))
	return cmd
}

func newSessionAttachmentsUploadCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:     "upload <session-id> <file>",
		Short:   "Upload a file to a session",
		Args:    exactTwoNonBlankArgs(),
		Example: "  compozy session attachments upload sess_1234 ./image.png",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.UploadSessionAttachment(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionAttachmentBundle(record))
		},
	}
}

func uploadSessionPromptAttachments(
	ctx context.Context,
	client sessionClientAPI,
	sessionID string,
	paths []string,
) ([]contract.PromptAttachmentRef, error) {
	var refs []contract.PromptAttachmentRef
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		record, err := client.UploadSessionAttachment(ctx, sessionID, path)
		if err != nil {
			return nil, err
		}
		refs = append(refs, contract.PromptAttachmentRef{
			ID: record.ID, Name: record.Name, MIMEType: record.MIMEType, Bytes: record.Bytes,
			SHA256: record.SHA256, Kind: record.Kind, Width: record.Width, Height: record.Height,
		})
	}
	return refs, nil
}
