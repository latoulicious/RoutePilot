package cli

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "github.com/google/uuid"
    "github.com/spf13/cobra"

    "github.com/latoulicious/RoutePilot/internal/adapters/db"
    dflags "github.com/latoulicious/RoutePilot/internal/domain/flags"
)

var (
    flagType   string
    flagDesc   string
    flagEnable bool
)

var flagCmd = &cobra.Command{
    Use:   "flag",
    Short: "Manage flags",
}

var flagCreateCmd = &cobra.Command{
    Use:   "create [tenant_id] [key]",
    Short: "Create a new flag",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        tenantStr := args[0]
        key := args[1]
        return createFlag(cmd.Context(), tenantStr, key, flagType, flagDesc, flagEnable)
    },
}

var flagListCmd = &cobra.Command{
    Use:   "list [tenant_id]",
    Short: "List flags for a tenant",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return listFlags(cmd.Context(), args[0])
    },
}

var flagEnableCmd = &cobra.Command{
    Use:   "enable [flag_id]",
    Short: "Enable a flag",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return setFlagEnabled(cmd.Context(), args[0], true)
    },
}

var flagDisableCmd = &cobra.Command{
    Use:   "disable [flag_id]",
    Short: "Disable a flag",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return setFlagEnabled(cmd.Context(), args[0], false)
    },
}

var flagRotateSaltCmd = &cobra.Command{
    Use:   "rotate-salt [flag_id]",
    Short: "Rotate a flag's salt",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return rotateFlagSalt(cmd.Context(), args[0])
    },
}

var ruleAddCmd = &cobra.Command{
    Use:   "rule add [flag_id]",
    Short: "Add a rule to a flag",
    Args:  cobra.ExactArgs(1),
}

var (
    rulePriority int
    ruleRollout  int
    ruleVariant  string
)

func init() {
    rootCmd.AddCommand(flagCmd)

    // flag create
    flagCmd.AddCommand(flagCreateCmd)
    flagCreateCmd.Flags().StringVar(&flagType, "type", "boolean", "Flag type: boolean|json")
    flagCreateCmd.Flags().StringVar(&flagDesc, "desc", "", "Description")
    flagCreateCmd.Flags().BoolVar(&flagEnable, "enabled", false, "Start enabled")

    // flag enable/disable
    flagCmd.AddCommand(flagEnableCmd)
    flagCmd.AddCommand(flagDisableCmd)

    // rotate salt
    flagCmd.AddCommand(flagRotateSaltCmd)

    // rule add
    flagCmd.AddCommand(ruleAddCmd)
    ruleAddCmd.Flags().IntVar(&rulePriority, "priority", 0, "Rule priority (lower is higher priority)")
    ruleAddCmd.Flags().IntVar(&ruleRollout, "rollout", 0, "Rollout percent 0..100")
    ruleAddCmd.Flags().StringVar(&ruleVariant, "variant", "{}", "Variant JSON value")
    ruleAddCmd.RunE = func(cmd *cobra.Command, args []string) error {
        return addRule(cmd.Context(), args[0], rulePriority, ruleRollout, ruleVariant)
    }
    // list
    flagCmd.AddCommand(flagListCmd)

    // rule list subcommand
    ruleListCmd := &cobra.Command{
        Use:   "rule list [flag_id]",
        Short: "List rules for a flag",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return listRules(cmd.Context(), args[0])
        },
    }
    flagCmd.AddCommand(ruleListCmd)
}

func createFlag(ctx context.Context, tenantStr, key, ftype, desc string, enabled bool) error {
    if databaseURL == "" {
        return fmt.Errorf("database URL is required. Set --database-url or DATABASE_URL")
    }
    tenantID, err := uuid.Parse(tenantStr)
    if err != nil {
        return fmt.Errorf("invalid tenant_id: %w", err)
    }
    if key == "" || !isValidKey(key) {
        return fmt.Errorf("invalid key: must be alphanumeric/underscore")
    }
    var typ dflags.FlagType
    switch strings.ToLower(ftype) {
    case string(dflags.FlagTypeBoolean):
        typ = dflags.FlagTypeBoolean
    case string(dflags.FlagTypeJSON):
        typ = dflags.FlagTypeJSON
    default:
        return fmt.Errorf("invalid type: %s", ftype)
    }

    pool, err := db.Connect(databaseURL)
    if err != nil { return err }
    defer pool.Close()
    queries := db.New(pool)
    repo := db.NewFlagRepositoryAdapter(queries)

    salt := uuid.New().String()
    flag := &dflags.Flag{
        ID:          uuid.New(),
        TenantID:    tenantID,
        Key:         key,
        Description: desc,
        Type:        typ,
        Enabled:     enabled,
        Salt:        salt,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }
    if err := repo.CreateFlag(ctx, flag); err != nil { return err }
    if outputJSON { return printJSON(flag) }
    fmt.Printf("✅ Flag created\n")
    fmt.Printf("   id: %s\n   key: %s\n   type: %s\n   enabled: %v\n   salt: %s\n", flag.ID, flag.Key, flag.Type, flag.Enabled, flag.Salt)
    return nil
}

func setFlagEnabled(ctx context.Context, flagIDStr string, enabled bool) error {
    if databaseURL == "" { return fmt.Errorf("database URL is required") }
    flagID, err := uuid.Parse(flagIDStr)
    if err != nil { return fmt.Errorf("invalid flag_id: %w", err) }
    pool, err := db.Connect(databaseURL)
    if err != nil { return err }
    defer pool.Close()
    queries := db.New(pool)
    repo := db.NewFlagRepositoryAdapter(queries)
    upd := dflags.FlagUpdates{ Enabled: &enabled }
    if err := repo.UpdateFlag(ctx, flagID, upd); err != nil { return err }
    if outputJSON { return printJSON(map[string]any{"flag_id": flagID, "enabled": enabled}) }
    fmt.Printf("✅ Flag %s\n", map[bool]string{true:"enabled", false:"disabled"}[enabled])
    return nil
}

func rotateFlagSalt(ctx context.Context, flagIDStr string) error {
    if databaseURL == "" { return fmt.Errorf("database URL is required") }
    flagID, err := uuid.Parse(flagIDStr)
    if err != nil { return fmt.Errorf("invalid flag_id: %w", err) }
    pool, err := db.Connect(databaseURL)
    if err != nil { return err }
    defer pool.Close()
    queries := db.New(pool)
    repo := db.NewFlagRepositoryAdapter(queries)
    newSalt := uuid.New().String()
    upd := dflags.FlagUpdates{ Salt: &newSalt }
    if err := repo.UpdateFlag(ctx, flagID, upd); err != nil { return err }
    if outputJSON { return printJSON(map[string]any{"flag_id": flagID, "new_salt": newSalt}) }
    fmt.Printf("✅ Salt rotated\n   new_salt: %s\n", newSalt)
    return nil
}

func addRule(ctx context.Context, flagIDStr string, priority, rollout int, variant string) error {
    if databaseURL == "" { return fmt.Errorf("database URL is required") }
    flagID, err := uuid.Parse(flagIDStr)
    if err != nil { return fmt.Errorf("invalid flag_id: %w", err) }
    if priority < 0 { return fmt.Errorf("priority must be >= 0") }
    if rollout < 0 || rollout > 100 { return fmt.Errorf("rollout must be 0..100") }
    var raw json.RawMessage
    if err := json.Unmarshal([]byte(variant), &raw); err != nil {
        return fmt.Errorf("variant must be valid JSON: %w", err)
    }

    pool, err := db.Connect(databaseURL)
    if err != nil { return err }
    defer pool.Close()
    queries := db.New(pool)
    repo := db.NewFlagRepositoryAdapter(queries)
    rule := &dflags.FlagRule{
        ID:       uuid.New(),
        FlagID:   flagID,
        Priority: priority,
        Rollout:  rollout,
        Variant:  raw,
    }
    if err := repo.CreateFlagRule(ctx, rule); err != nil { return err }
    if outputJSON {
        return printJSON(map[string]interface{}{"id": rule.ID, "priority": rule.Priority, "rollout": rule.Rollout})
    }
    fmt.Printf("✅ Rule added\n   id: %s\n   priority: %d\n   rollout: %d\n", rule.ID, rule.Priority, rule.Rollout)
    return nil
}

func listRules(ctx context.Context, flagIDStr string) error {
    if databaseURL == "" { return fmt.Errorf("database URL is required") }
    flagID, err := uuid.Parse(flagIDStr)
    if err != nil { return fmt.Errorf("invalid flag_id: %w", err) }
    pool, err := db.Connect(databaseURL)
    if err != nil { return err }
    defer pool.Close()
    queries := db.New(pool)
    repo := db.NewFlagRepositoryAdapter(queries)
    rules, err := repo.GetFlagRules(ctx, flagID)
    if err != nil { return err }
    if outputJSON { return printJSON(rules) }
    for _, r := range rules {
        fmt.Printf("%d\t%d\t%s\n", r.Priority, r.Rollout, string(r.Variant))
    }
    return nil
}

func listFlags(ctx context.Context, tenantStr string) error {
    if databaseURL == "" { return fmt.Errorf("database URL is required") }
    tenantID, err := uuid.Parse(tenantStr)
    if err != nil { return fmt.Errorf("invalid tenant_id: %w", err) }
    pool, err := db.Connect(databaseURL)
    if err != nil { return err }
    defer pool.Close()
    queries := db.New(pool)
    repo := db.NewFlagRepositoryAdapter(queries)
    flags, err := repo.ListFlagsByTenant(ctx, tenantID)
    if err != nil { return err }
    if outputJSON {
        return printJSON(flags)
    }
    for _, f := range flags {
        fmt.Printf("%s\t%s\t%s\t%v\n", f.ID, f.Key, f.Type, f.Enabled)
    }
    return nil
}

func isValidKey(s string) bool {
    for _, r := range s {
        if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
            return false
        }
    }
    return len(s) > 0
}
