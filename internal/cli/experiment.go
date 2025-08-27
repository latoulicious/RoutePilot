package cli

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/google/uuid"
    "github.com/spf13/cobra"

    "github.com/latoulicious/RoutePilot/internal/adapters/db"
    dflags "github.com/latoulicious/RoutePilot/internal/domain/flags"
)

var expCmd = &cobra.Command{
    Use:   "exp",
    Short: "Manage experiments",
}

var expCreateCmd = &cobra.Command{
    Use:   "create [tenant_id] [flag_id] --key=<key> --traffic=<0..100>",
    Short: "Create an experiment",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        tenantStr, flagStr := args[0], args[1]
        key, _ := cmd.Flags().GetString("key")
        traffic, _ := cmd.Flags().GetInt("traffic")
        return expCreate(cmd.Context(), tenantStr, flagStr, key, traffic)
    },
}

var expAddVariantCmd = &cobra.Command{
    Use:   "add-variant [exp_id] [name] [weight] [json_config]",
    Short: "Add a variant to an experiment",
    Args:  cobra.ExactArgs(4),
    RunE: func(cmd *cobra.Command, args []string) error {
        return expAddVariant(cmd.Context(), args[0], args[1], args[2], args[3])
    },
}

var expStartCmd = &cobra.Command{
    Use:   "start [exp_id]",
    Short: "Start an experiment",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return expSetStatus(cmd.Context(), args[0], dflags.ExperimentStatusRunning)
    },
}

var expStopCmd = &cobra.Command{
    Use:   "stop [exp_id]",
    Short: "Stop an experiment",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return expSetStatus(cmd.Context(), args[0], dflags.ExperimentStatusStopped)
    },
}

func init() {
    rootCmd.AddCommand(expCmd)
    expCmd.AddCommand(expCreateCmd)
    expCreateCmd.Flags().String("key", "", "Experiment key")
    expCreateCmd.Flags().Int("traffic", 0, "Traffic percent 0..100")
    expCreateCmd.MarkFlagRequired("key")
    expCreateCmd.MarkFlagRequired("traffic")

    expCmd.AddCommand(expAddVariantCmd)
    expCmd.AddCommand(expStartCmd)
    expCmd.AddCommand(expStopCmd)
}

func expCreate(ctx context.Context, tenantStr, flagStr, key string, traffic int) error {
    if databaseURL == "" { return fmt.Errorf("database URL is required") }
    tenantID, err := uuid.Parse(tenantStr)
    if err != nil { return fmt.Errorf("invalid tenant_id: %w", err) }
    flagID, err := uuid.Parse(flagStr)
    if err != nil { return fmt.Errorf("invalid flag_id: %w", err) }
    if key == "" || !isValidKey(key) { return fmt.Errorf("invalid key") }
    if traffic < 0 || traffic > 100 { return fmt.Errorf("traffic must be 0..100") }

    pool, err := db.Connect(databaseURL)
    if err != nil { return err }
    defer pool.Close()
    queries := db.New(pool)
    repo := db.NewExperimentRepositoryAdapter(queries)
    exp := &dflags.Experiment{ID: uuid.New(), TenantID: tenantID, FlagID: flagID, Key: key, Traffic: traffic}
    if err := repo.CreateExperiment(ctx, exp); err != nil { return err }
    if outputJSON { return printJSON(exp) }
    fmt.Printf("✅ Experiment created\n   id: %s\n   key: %s\n   traffic: %d\n", exp.ID, exp.Key, exp.Traffic)
    return nil
}

func expAddVariant(ctx context.Context, expIDStr, name, weightStr, jsonConfig string) error {
    if databaseURL == "" { return fmt.Errorf("database URL is required") }
    expID, err := uuid.Parse(expIDStr)
    if err != nil { return fmt.Errorf("invalid exp_id: %w", err) }
    var cfg json.RawMessage
    if err := json.Unmarshal([]byte(jsonConfig), &cfg); err != nil { return fmt.Errorf("invalid JSON: %w", err) }
    var weight int
    if _, err := fmt.Sscanf(weightStr, "%d", &weight); err != nil { return fmt.Errorf("invalid weight: %w", err) }
    if weight < 0 { return fmt.Errorf("weight must be >= 0") }

    pool, err := db.Connect(databaseURL)
    if err != nil { return err }
    defer pool.Close()
    queries := db.New(pool)
    repo := db.NewExperimentRepositoryAdapter(queries)
    v := &dflags.ExperimentVariant{ID: uuid.New(), ExperimentID: expID, Name: name, Weight: weight, Value: cfg}
    if err := repo.CreateExperimentVariant(ctx, v); err != nil { return err }
    if outputJSON { return printJSON(v) }
    fmt.Printf("✅ Variant added\n   id: %s\n   name: %s\n   weight: %d\n", v.ID, v.Name, v.Weight)
    return nil
}

func expSetStatus(ctx context.Context, expIDStr string, status dflags.ExperimentStatus) error {
    if databaseURL == "" { return fmt.Errorf("database URL is required") }
    expID, err := uuid.Parse(expIDStr)
    if err != nil { return fmt.Errorf("invalid exp_id: %w", err) }
    pool, err := db.Connect(databaseURL)
    if err != nil { return err }
    defer pool.Close()
    queries := db.New(pool)
    repo := db.NewExperimentRepositoryAdapter(queries)
    if err := repo.UpdateExperiment(ctx, expID, status); err != nil { return err }
    if outputJSON { return printJSON(map[string]string{"status": string(status)}) }
    fmt.Printf("✅ Status updated: %s\n", status)
    return nil
}

