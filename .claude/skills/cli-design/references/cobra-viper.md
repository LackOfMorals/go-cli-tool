# Cobra + Viper Reference

## Project structure

```
myapp/
├── main.go              # calls cmd.Execute(), nothing else
├── cmd/
│   ├── root.go          # rootCmd, initConfig, PersistentPreRunE
│   ├── deploy.go        # one file per subcommand
│   ├── list.go
│   └── version.go
└── internal/
    └── output/          # output formatters (text, json, yaml)
```

Keep `main.go` to a minimum — its only job is:
```go
func main() {
    if err := cmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

---

## Root command: the canonical pattern

```go
// cmd/root.go
var cfgFile string

var rootCmd = &cobra.Command{
    Use:               "myapp",
    Short:             "One-line description",
    SilenceErrors:     true,  // prevents cobra printing errors twice
    SilenceUsage:      true,  // prevents usage being dumped on RunE errors
    PersistentPreRunE: initRuntime,
}

func Execute() error {
    return rootCmd.Execute()
}

func init() {
    cobra.OnInitialize(initConfig)

    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
    rootCmd.PersistentFlags().StringP("format", "f", "text", "Output format: text|json|yaml")
    rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose output")
    rootCmd.PersistentFlags().Bool("no-color", false, "Disable color output")

    // Bind persistent flags to viper immediately after defining them
    viper.BindPFlag("format",   rootCmd.PersistentFlags().Lookup("format"))
    viper.BindPFlag("verbose",  rootCmd.PersistentFlags().Lookup("verbose"))
    viper.BindPFlag("no-color", rootCmd.PersistentFlags().Lookup("no-color"))
}

// initConfig: loads config file + env vars. Called by cobra.OnInitialize
// before flag parsing completes — cannot return errors from here.
func initConfig() {
    if cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    } else {
        home, _ := os.UserHomeDir()
        viper.AddConfigPath(".")
        viper.AddConfigPath(home + "/.config/myapp")
        viper.SetConfigName("config")
        viper.SetConfigType("yaml")
    }

    viper.SetEnvPrefix("MYAPP")
    viper.AutomaticEnv()
    // Maps --my-flag → MYAPP_MY_FLAG (hyphen → underscore)
    viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

    if err := viper.ReadInConfig(); err == nil && viper.GetBool("verbose") {
        fmt.Fprintln(os.Stderr, "Using config:", viper.ConfigFileUsed())
    }
}

// initRuntime: error-returning pre-run hook for auth checks, validation, etc.
// Runs before every command's RunE. Inherited by all subcommands UNLESS
// a subcommand declares its own PersistentPreRunE — in that case the parent's
// will NOT run automatically. Call it explicitly if needed.
func initRuntime(cmd *cobra.Command, args []string) error {
    // e.g. validate credentials, apply NO_COLOR, configure logger
    if os.Getenv("NO_COLOR") != "" || viper.GetBool("no-color") {
        color.NoColor = true
    }
    return nil
}
```

---

## Critical: always read config from viper, never from flag variables

When `viper.BindPFlag` is used, **the original flag variable is NOT updated** by
viper's precedence chain. The variable only reflects the value passed directly on
the command line. To get the final resolved value (flag > env > config > default),
always use viper getters:

```go
// WRONG — misses env vars and config file values
format, _ := cmd.Flags().GetString("format")

// CORRECT — viper resolves the full precedence chain
format := viper.GetString("format")
```

---

## Subcommand pattern

```go
// cmd/deploy.go
var deployCmd = &cobra.Command{
    Use:   "deploy <environment>",
    Short: "Deploy to the specified environment",
    Args:  cobra.ExactArgs(1),
    RunE:  runDeploy,
}

func init() {
    rootCmd.AddCommand(deployCmd)

    deployCmd.Flags().StringP("image", "i", "", "Container image tag")
    deployCmd.Flags().Bool("dry-run", false, "Preview changes without applying")
    viper.BindPFlag("deploy.image",   deployCmd.Flags().Lookup("image"))
    viper.BindPFlag("deploy.dry-run", deployCmd.Flags().Lookup("dry-run"))
}

func runDeploy(cmd *cobra.Command, args []string) error {
    env := args[0]
    image   := viper.GetString("deploy.image")
    dryRun  := viper.GetBool("deploy.dry-run")

    // Write output through cmd.OutOrStdout() / cmd.ErrOrStderr()
    // so tests can capture and assert on it
    out := cmd.OutOrStdout()

    if dryRun {
        fmt.Fprintf(out, "[dry-run] Would deploy %s to %s\n", image, env)
        return nil
    }
    return deploy(env, image)
}
```

---

## PersistentPreRunE inheritance pitfall

If a subcommand defines its own `PersistentPreRunE`, Cobra will **not** call the
parent's. You must call it explicitly:

```go
var subCmd = &cobra.Command{
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        // Must manually chain parent hook
        if err := initRuntime(cmd, args); err != nil {
            return err
        }
        // subcommand-specific pre-run logic here
        return validateSubCmdConfig()
    },
}
```

---

## Error handling and exit codes

Cobra prints the error returned by `RunE` unless `SilenceErrors: true` is set.
With `SilenceErrors: true`, handle printing yourself:

```go
func Execute() error {
    err := rootCmd.Execute()
    if err != nil {
        // Print in your own error format (respects --format json)
        printError(os.Stderr, err)
        os.Exit(exitCodeFor(err))
    }
    return nil
}
```

Use sentinel errors or typed errors to drive exit codes:

```go
type CLIError struct {
    Code    int
    Message string
    Err     error
}

func exitCodeFor(err error) int {
    var cliErr *CLIError
    if errors.As(err, &cliErr) {
        return cliErr.Code
    }
    return 1
}
```

---

## Shell completion

Cobra generates completion scripts for bash, zsh, fish, and PowerShell automatically.
Add a `completion` subcommand:

```go
// This is built-in — no extra code needed with recent Cobra versions
rootCmd.CompletionOptions.DisableDefaultCmd = false
```

Users run:
```bash
myapp completion bash >> ~/.bashrc
myapp completion zsh  >> ~/.zshrc
```

---

## Version flag

```go
rootCmd.Version = "1.2.3"
rootCmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
```

This gives `myapp --version` and `myapp -v` (if `-v` isn't used for verbose — use
`-V` for version in that case by overriding the template and using a custom flag).

---

## Testability

- Use `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` instead of `os.Stdout` / `os.Stderr` in all commands
- In tests, call `rootCmd.SetOut(buf)` and `rootCmd.SetErr(errBuf)` to capture output
- Instantiate a fresh `viper.New()` per test — the global singleton bleeds state between tests
- Pass the viper instance through a context or command annotations rather than using globals
