package cli

import "fmt"

func sessionAttachmentBundle(info SessionAttachmentRecord) outputBundle {
	return outputBundle{
		jsonValue: info,
		human: func() (string, error) {
			return renderHumanSection("Session Attachment", []keyValue{
				{Label: "ID", Value: stringOrDash(info.ID)},
				{Label: cliNameValue, Value: stringOrDash(info.Name)},
				{Label: "MIME Type", Value: stringOrDash(info.MIMEType)},
				{Label: "Bytes", Value: fmt.Sprint(info.Bytes)},
				{Label: "SHA-256", Value: stringOrDash(info.SHA256)},
				{Label: cliKindValue, Value: stringOrDash(info.Kind)},
				{Label: "Width", Value: fmt.Sprint(info.Width)},
				{Label: "Height", Value: fmt.Sprint(info.Height)},
				{Label: "Created At", Value: stringOrDash(formatTime(info.CreatedAt))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("session_attachment", []string{
				"id", sessionNameKey, "mime_type", cliBytesKey, "sha256", cliKindKey,
				"width", "height", automationCreatedAtKey,
			}, []string{
				info.ID,
				info.Name,
				info.MIMEType,
				fmt.Sprint(info.Bytes),
				info.SHA256,
				info.Kind,
				fmt.Sprint(info.Width),
				fmt.Sprint(info.Height),
				formatTime(info.CreatedAt),
			}), nil
		},
	}
}
