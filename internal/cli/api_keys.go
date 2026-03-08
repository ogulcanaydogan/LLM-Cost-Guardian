package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	keyauth "github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/auth"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
	"github.com/spf13/cobra"
)

var apiKeysCmd = &cobra.Command{
	Use:   "api-keys",
	Short: "Manage tenant API keys",
}

var apiKeysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an API key for a tenant",
	RunE:  runAPIKeyCreate,
}

var apiKeysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys",
	RunE:  runAPIKeyList,
}

var apiKeysRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke an API key",
	RunE:  runAPIKeyRevoke,
}

func init() {
	rootCmd.AddCommand(apiKeysCmd)
	apiKeysCmd.AddCommand(apiKeysCreateCmd)
	apiKeysCmd.AddCommand(apiKeysListCmd)
	apiKeysCmd.AddCommand(apiKeysRevokeCmd)

	apiKeysCreateCmd.Flags().String("tenant", "", "Tenant slug")
	apiKeysCreateCmd.Flags().String("name", "default", "API key name")
	_ = apiKeysCreateCmd.MarkFlagRequired("tenant")

	apiKeysListCmd.Flags().String("tenant", "", "Tenant slug filter")

	apiKeysRevokeCmd.Flags().String("id", "", "API key id")
	_ = apiKeysRevokeCmd.MarkFlagRequired("id")
}

func runAPIKeyCreate(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	tenant, _ := cmd.Flags().GetString("tenant")
	name, _ := cmd.Flags().GetString("name")

	rawKey, prefix, hash, err := keyauth.GenerateAPIKey()
	if err != nil {
		return err
	}

	_, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	key := &model.APIKey{
		Tenant:    tenant,
		Name:      name,
		KeyPrefix: prefix,
		KeyHash:   hash,
		Status:    model.APIKeyStatusActive,
	}
	if err := store.CreateAPIKey(commandContext(cmd), key); err != nil {
		return fmt.Errorf("create api key: %w", err)
	}

	fmt.Printf("API key created:\n")
	fmt.Printf("  Tenant:    %s\n", tenant)
	fmt.Printf("  Name:      %s\n", name)
	fmt.Printf("  ID:        %s\n", key.ID)
	fmt.Printf("  Prefix:    %s\n", prefix)
	fmt.Printf("  Raw Key:   %s\n", rawKey)
	return nil
}

func runAPIKeyList(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	tenant, _ := cmd.Flags().GetString("tenant")

	_, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	keys, err := store.ListAPIKeys(commandContext(cmd), tenant)
	if err != nil {
		return fmt.Errorf("list api keys: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tTENANT\tNAME\tPREFIX\tSTATUS\tLAST USED\n")
	for _, key := range keys {
		lastUsed := "-"
		if key.LastUsedAt != nil {
			lastUsed = key.LastUsedAt.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", key.ID, key.Tenant, key.Name, key.KeyPrefix, key.Status, lastUsed)
	}
	w.Flush()
	return nil
}

func runAPIKeyRevoke(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	id, _ := cmd.Flags().GetString("id")

	_, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.RevokeAPIKey(commandContext(cmd), id); err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}

	fmt.Printf("API key revoked: %s\n", id)
	return nil
}
