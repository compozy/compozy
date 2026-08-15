package cli

import "github.com/spf13/cobra"

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
