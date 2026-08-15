package cli

import "fmt"

func sessionAttachmentBundle(info SessionAttachmentRecord) outputBundle {
	return outputBundle{
		jsonValue: info,
		human: func() (string, error) {
			return renderHumanSection("Session Attachment", []keyValue{
				{Label: "ID", Value: stringOrDash(info.ID)},
				{Label: "Name", Value: stringOrDash(info.Name)},
				{Label: "MIME Type", Value: stringOrDash(info.MIMEType)},
				{Label: "Bytes", Value: fmt.Sprint(info.Bytes)},
				{Label: "SHA-256", Value: stringOrDash(info.SHA256)},
				{Label: "Kind", Value: stringOrDash(info.Kind)},
				{Label: "Width", Value: fmt.Sprint(info.Width)},
				{Label: "Height", Value: fmt.Sprint(info.Height)},
				{Label: "Created At", Value: fmt.Sprint(info.CreatedAt)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("session_attachment", []string{
				"id", "name", "mime_type", "bytes", "sha256", "kind", "width", "height", "created_at",
			}, []string{
				info.ID,
				info.Name,
				info.MIMEType,
				fmt.Sprint(info.Bytes),
				info.SHA256,
				info.Kind,
				fmt.Sprint(info.Width),
				fmt.Sprint(info.Height),
				fmt.Sprint(info.CreatedAt),
			}), nil
		},
	}
}
