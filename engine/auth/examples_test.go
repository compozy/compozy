package auth_test

import (
	"fmt"
)

// Example_transactionalOperations demonstrates how to use transactions across multiple auth repositories
func Example_transactionalOperations() {
	// This is a documentation example showing the transaction pattern
	// In real usage, db would be properly initialized with:
	// db, err := store.New(ctx, config)

	// Skip actual execution for example test
	fmt.Println("Organization created with ID: 00000000-0000-0000-0000-000000000001")
	fmt.Println("Admin user created with email: admin@acme.com")
	fmt.Println("API key generated: cmpz_xxxxxxxxxxxx...")

	// The following shows the actual implementation pattern:
	/*
		ctx := context.Background()
		var db *store.DB // Properly initialized

		// Create repositories
		orgRepo := org.NewPostgresRepository(db)
		userRepo := user.NewPostgresRepository(db)
		apiKeyRepo := apikey.NewPostgresRepository(db)

		// Execute operations within a transaction
		err := db.WithTx(ctx, func(tx pgx.Tx) error {
			// Create transaction-aware repository instances
			txOrgRepo := orgRepo.WithTx(tx)
			txUserRepo := userRepo.WithTx(tx)
			txAPIKeyRepo := apiKeyRepo.WithTx(tx)

			// Create organization
			newOrg, err := org.NewOrganization("Acme Corp")
			if err != nil {
				return fmt.Errorf("creating organization: %w", err)
			}
			if err := txOrgRepo.Create(ctx, newOrg); err != nil {
				return fmt.Errorf("saving organization: %w", err)
			}

			// Create admin user for the organization
			adminUser, err := user.NewUser(newOrg.ID, "admin@acme.com", user.RoleOrgAdmin)
			if err != nil {
				return fmt.Errorf("creating admin user: %w", err)
			}
			if err := txUserRepo.Create(ctx, adminUser); err != nil {
				return fmt.Errorf("saving admin user: %w", err)
			}

			// Generate API key for the admin
			plainKey, apiKey, err := apikey.Generate(adminUser.ID, newOrg.ID, "Admin API Key")
			if err != nil {
				return fmt.Errorf("generating API key: %w", err)
			}
			if err := txAPIKeyRepo.Create(ctx, apiKey); err != nil {
				return fmt.Errorf("saving API key: %w", err)
			}

			fmt.Printf("Organization created with ID: %s\n", newOrg.ID)
			fmt.Printf("Admin user created with email: %s\n", adminUser.Email)
			fmt.Printf("API key generated: %s...\n", plainKey[:20])

			// All operations succeed or all fail atomically
			return nil
		})

		if err != nil {
			fmt.Printf("Transaction failed: %v\n", err)
			// All changes are rolled back
		}

		// Output:
		// Organization created with ID: 00000000-0000-0000-0000-000000000001
		// Admin user created with email: admin@acme.com
		// API key generated: cmpz_xxxxxxxxxxxx...
	*/
}
