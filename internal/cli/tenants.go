package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
	"github.com/spf13/cobra"
)

var tenantsCmd = &cobra.Command{
	Use:   "tenants",
	Short: "Manage tenants",
}

var tenantsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a tenant",
	RunE:  runTenantCreate,
}

var tenantsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tenants",
	RunE:  runTenantList,
}

var tenantsDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable a tenant",
	RunE:  runTenantDisable,
}

func init() {
	rootCmd.AddCommand(tenantsCmd)
	tenantsCmd.AddCommand(tenantsCreateCmd)
	tenantsCmd.AddCommand(tenantsListCmd)
	tenantsCmd.AddCommand(tenantsDisableCmd)

	tenantsCreateCmd.Flags().String("slug", "", "Tenant slug")
	tenantsCreateCmd.Flags().String("name", "", "Tenant display name")
	_ = tenantsCreateCmd.MarkFlagRequired("slug")

	tenantsDisableCmd.Flags().String("slug", "", "Tenant slug")
	_ = tenantsDisableCmd.MarkFlagRequired("slug")
}

func runTenantCreate(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	slug, _ := cmd.Flags().GetString("slug")
	name, _ := cmd.Flags().GetString("name")

	_, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	tenant := &model.Tenant{
		Slug:   slug,
		Name:   name,
		Status: model.TenantStatusActive,
	}
	if err := store.CreateTenant(commandContext(cmd), tenant); err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}

	fmt.Printf("Tenant created:\n")
	fmt.Printf("  Slug:    %s\n", tenant.Slug)
	fmt.Printf("  Name:    %s\n", tenant.Name)
	fmt.Printf("  Status:  %s\n", tenant.Status)
	return nil
}

func runTenantList(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	_, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	tenants, err := store.ListTenants(commandContext(cmd))
	if err != nil {
		return fmt.Errorf("list tenants: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "SLUG\tNAME\tSTATUS\tUPDATED\n")
	for _, tenant := range tenants {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", tenant.Slug, tenant.Name, tenant.Status, tenant.UpdatedAt.Format("2006-01-02 15:04"))
	}
	w.Flush()
	return nil
}

func runTenantDisable(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	slug, _ := cmd.Flags().GetString("slug")

	_, store, err := initTracker(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.DisableTenant(commandContext(cmd), slug); err != nil {
		return fmt.Errorf("disable tenant: %w", err)
	}

	fmt.Printf("Tenant disabled: %s\n", slug)
	return nil
}
