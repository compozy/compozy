- [ ] In cli/auth/tui/components/error.go around lines 40 to 48, the error message is
      duplicated by rendering it once as a styled error and again inside the error box
      when ShowDetails is true. To fix this, remove the repeated error message inside
      the details view and instead render additional context or details about the
      error in the error box to avoid duplication.
