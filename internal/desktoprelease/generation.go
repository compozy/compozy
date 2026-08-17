package desktoprelease

import "fmt"

func DecodeGeneration(contents []byte) (Generation, error) {
	generation, err := decodeStrictJSON[Generation](contents, "generation")
	if err != nil {
		return Generation{}, err
	}
	if generation.OperationID == "" || generation.Operation == "" {
		return Generation{}, fmt.Errorf("desktop release: generation audit identity is incomplete")
	}
	if generation.Operation != operationPublish && generation.Operation != operationRepair {
		return Generation{}, fmt.Errorf("desktop release: unsupported generation operation %q", generation.Operation)
	}
	if err := ValidateVersion(generation.Version); err != nil {
		return Generation{}, err
	}
	if err := ValidateVersion(generation.MinAppVersion); err != nil {
		return Generation{}, fmt.Errorf("desktop release: generation min_app_version: %w", err)
	}
	if generation.PublishedAt.IsZero() {
		return Generation{}, fmt.Errorf("desktop release: generation published_at is required")
	}
	if generation.Operation == operationRepair && generation.SourceCommit == "" {
		return Generation{}, fmt.Errorf("desktop release: repair generation source_commit is required")
	}
	return generation, nil
}
