package tool

import (
	"testing"
)

func TestNewContext(t *testing.T) {
	ctx := NewContext()

	if ctx == nil {
		t.Fatal("Expected non-nil context")
	}

	if ctx.Args == nil {
		t.Error("Expected Args to be initialized")
	}

	if ctx.Flags == nil {
		t.Error("Expected Flags to be initialized")
	}

	if ctx.EnvVars == nil {
		t.Error("Expected EnvVars to be initialized")
	}

	if ctx.Config == nil {
		t.Error("Expected Config to be initialized")
	}
}

func TestContextWithArgs(t *testing.T) {
	ctx := NewContext().WithArgs([]string{"arg1", "arg2"})

	if len(ctx.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(ctx.Args))
	}

	if ctx.Args[0] != "arg1" {
		t.Errorf("Expected arg1, got %s", ctx.Args[0])
	}
}

func TestContextWithFlags(t *testing.T) {
	ctx := NewContext().WithFlags(map[string]string{"flag1": "value1"})

	if ctx.GetFlag("flag1") != "value1" {
		t.Errorf("Expected flag1 value, got %s", ctx.GetFlag("flag1"))
	}

	if ctx.GetFlag("nonexistent") != "" {
		t.Error("Expected empty string for nonexistent flag")
	}
}

func TestContextHasFlag(t *testing.T) {
	ctx := NewContext().WithFlags(map[string]string{"exists": "value"})

	if !ctx.HasFlag("exists") {
		t.Error("Expected flag to exist")
	}

	if ctx.HasFlag("notexists") {
		t.Error("Expected flag to not exist")
	}
}

func TestContextGetArg(t *testing.T) {
	ctx := NewContext().WithArgs([]string{"first", "second", "third"})

	if ctx.GetArg(0) != "first" {
		t.Errorf("Expected first, got %s", ctx.GetArg(0))
	}

	if ctx.GetArg(1) != "second" {
		t.Errorf("Expected second, got %s", ctx.GetArg(1))
	}

	if ctx.GetArg(5) != "" {
		t.Error("Expected empty string for out of bounds index")
	}
}

func TestContextWithEnvVars(t *testing.T) {
	ctx := NewContext().WithEnvVars(map[string]string{"MY_VAR": "my_value"})

	if ctx.GetEnvVar("MY_VAR") != "my_value" {
		t.Errorf("Expected my_value, got %s", ctx.GetEnvVar("MY_VAR"))
	}

	if ctx.GetEnvVar("UNDEFINED") != "" {
		t.Error("Expected empty string for undefined env var")
	}
}

func TestContextWithConfig(t *testing.T) {
	ctx := NewContext().WithConfig(map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	})

	if ctx.GetConfig("key1") != "value1" {
		t.Errorf("Expected value1, got %v", ctx.GetConfig("key1"))
	}

	if ctx.GetConfigInt("key2", 0) != 42 {
		t.Errorf("Expected 42, got %v", ctx.GetConfigInt("key2", 0))
	}

	if ctx.GetConfigString("key3", "default") != "default" {
		t.Error("Expected default for missing key")
	}

	if ctx.GetConfigBool("key4", true) != true {
		t.Error("Expected default for missing key")
	}
}

func TestContextMergeFlags(t *testing.T) {
	ctx := NewContext().WithFlags(map[string]string{"existing": "value1"})
	ctx.MergeFlags(map[string]string{"new": "value2"})

	if ctx.GetFlag("existing") != "value1" {
		t.Error("Expected existing flag to be preserved")
	}

	if ctx.GetFlag("new") != "value2" {
		t.Error("Expected new flag to be added")
	}
}

func TestContextMergeConfig(t *testing.T) {
	ctx := NewContext().WithConfig(map[string]interface{}{"key1": "value1"})
	ctx.MergeConfig(map[string]interface{}{"key2": "value2"})

	if ctx.GetConfig("key1") != "value1" {
		t.Error("Expected existing config to be preserved")
	}

	if ctx.GetConfig("key2") != "value2" {
		t.Error("Expected new config to be added")
	}
}
