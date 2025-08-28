package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

const version = "0.2.1"

var (
	projectRoot string
	cargoHome   string
	rustcPath   string
)

// Invocation represents a single build step from the Cargo build plan.
type Invocation struct {
	Number       int
	PackageName  string          `json:"package_name"`
	PackageVersion string          `json:"package_version"`
	TargetKind   []string        `json:"target_kind"`
	Kind         *string         `json:"kind"`
	CompileMode  string          `json:"compile_mode"`
	Deps         []int           `json:"deps"`
	Outputs      []string        `json:"outputs"`
	Links        map[string]string `json:"links"`
	Program      string          `json:"program"`
	Args         []string        `json:"args"`
	Env          map[string]string `json:"env"`
	Cwd          string          `json:"cwd"`
}

// CustomBuildDirectives captures directives from build script output.
type CustomBuildDirectives struct {
	RustcFlags []string
	EnvVars    map[string]string
}

// NewCustomBuildDirectives parses the output of a build script.
func NewCustomBuildDirectives(output string) *CustomBuildDirectives {
	lines := strings.Split(output, "\n")
	ignored := map[string]bool{"rerun-if-changed": true, "rerun-if-env-changed": true}
	directives := &CustomBuildDirectives{
		RustcFlags: []string{},
		EnvVars:    make(map[string]string),
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "cargo:") {
			continue
		}
		line = strings.TrimPrefix(line, "cargo:")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			log.Printf("Warning: Malformed build script output line (no '='): %s", line)
			continue
		}
		key, value := parts[0], parts[1]

		if ignored[key] {
			continue
		}

		switch key {
		case "rustc-cfg":
			directives.RustcFlags = append(directives.RustcFlags, "--cfg", value)
		case "rustc-check-cfg":
			directives.RustcFlags = append(directives.RustcFlags, "--check-cfg", value)
		case "rustc-link-lib":
			directives.RustcFlags = append(directives.RustcFlags, "-l", value)
		case "rustc-link-arg":
			directives.RustcFlags = append(directives.RustcFlags, "-C", "link-arg="+value)
		case "rustc-link-search":
			if strings.Contains(value, "=") {
				pathParts := strings.SplitN(value, "=", 2)
				// kind := pathParts[0]
				path := pathParts[1]
				directives.RustcFlags = append(directives.RustcFlags, "-L", path)
			} else {
				directives.RustcFlags = append(directives.RustcFlags, "-L", value)
			}
		case "rustc-env":
			kv := strings.SplitN(value, "=", 2)
			if len(kv) == 2 {
				directives.EnvVars[kv[0]] = kv[1]
			}
		default:
			log.Printf("Warning: Unknown build script output line: %s", line)
		}
	}
	return directives
}

// apply modifies the command and environment based on the directives.
func (d *CustomBuildDirectives) Apply(cmd *exec.Cmd) {
	cmd.Args = append(cmd.Args, d.RustcFlags...)
	for k, v := range d.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
}

// deepReplace recursively replaces strings in a map, slice, or string.
func deepReplace(data interface{}, replacements map[string]string) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			result[deepReplace(key, replacements).(string)] = deepReplace(value, replacements)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = deepReplace(item, replacements)
		}
		return result
	case string:
		s := v
		for old, new := range replacements {
			s = strings.ReplaceAll(s, old, new)
		}
		return s
	default:
		return v
	}
}

// resolveInvocationOrder performs a simple topological sort to order invocations.
func resolveInvocationOrder(invocations []Invocation) []Invocation {
	var ordered []Invocation
	todo := make(map[int]Invocation)
	for _, inv := range invocations {
		todo[inv.Number] = inv
	}
	satisfied := make(map[int]bool)

	for len(todo) > 0 {
		found := false
		for num, inv := range todo {
			depsSatisfied := true
			for _, dep := range inv.Deps {
				if !satisfied[dep] {
					depsSatisfied = false
					break
				}
			}
			if depsSatisfied {
				ordered = append(ordered, inv)
				satisfied[num] = true
				delete(todo, num)
				found = true
				break
			}
		}
		if !found {
			log.Fatal("Could not resolve invocation order due to circular dependencies or missing deps.")
		}
	}

	return ordered
}

// findRustc tries to locate the rustc executable.
func findRustc() (string, error) {
	if path, ok := syscall.Getenv("RUSTC"); ok && path != "" {
		log.Printf("Found rustc at %s using RUSTC environment variable.", path)
		return path, nil
	}
	
	if rustup, err := exec.LookPath("rustup"); err == nil {
		log.Printf("Using rustup at %s to find rustc.", rustup)
		cmd := exec.Command(rustup, "which", "rustc")
		cmd.Dir = "/"
		cmd.Env = os.Environ()
		if output, err := cmd.CombinedOutput(); err == nil {
			path := strings.TrimSpace(string(output))
			log.Printf("Found rustc at %s using rustup.", path)
			return path, nil
		}
	}

	// Fallback to PATH if rustup is not available
	if path, err := exec.LookPath("rustc"); err == nil {
		log.Printf("Found rustc at %s using PATH.", path)
		return path, nil
	}
	log.Printf("Warning: Could not find rustc using RUSTC or rustup. Falling back to 'rustc' in PATH.")
	return "rustc", nil // Fallback
}

// patch mode modifies the build plan JSON file in place.
func patch(buildPlanPath string, replacements map[string]string) {
	log.Printf("Patching build plan file: %s", buildPlanPath)

	// Don't patch RUSTC in the file, it's a runtime variable
	rustcPlaceholder := "{{RUSTC}}"
	delete(replacements, rustcPlaceholder)

	revReplacements := make(map[string]string)
	for k, v := range replacements {
		revReplacements[v] = k
	}

	data, err := os.ReadFile(buildPlanPath)
	if err != nil {
		log.Fatalf("Failed to read build plan file: %v", err)
	}

	var buildPlan map[string]interface{}
	if err := json.Unmarshal(data, &buildPlan); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	invocations, ok := buildPlan["invocations"].([]interface{})
	if !ok {
		log.Fatalf("%s does not look like a Cargo build plan file.", buildPlanPath)
	}

	patchedInvocations := make([]map[string]interface{}, len(invocations))
	for i, invRaw := range invocations {
		inv, ok := invRaw.(map[string]interface{})
		if !ok {
			log.Fatalf("Invalid invocation format.")
		}

		// Patching `program`
		if program, ok := inv["program"].(string); ok && program == "rustc" {
			inv["program"] = rustcPlaceholder
		}

		// Patching `env`
		if env, ok := inv["env"].(map[string]interface{}); ok {
			delete(env, "CARGO")
			delete(env, "PROJECT_ROOT")
			delete(env, "CARGO_HOME")
			delete(env, "RUSTC")
		}

		// Patching `args`
		if args, ok := inv["args"].([]interface{}); ok {
			var newArgs []interface{}
			for _, arg := range args {
				if argStr, ok := arg.(string); ok && strings.HasPrefix(argStr, "--diagnostic-width") {
					continue
				}
				newArgs = append(newArgs, arg)
			}
			inv["args"] = newArgs
		}

		// Perform deep replacement for all other values
		patchedInv := deepReplace(inv, revReplacements).(map[string]interface{})
		patchedInvocations[i] = patchedInv
	}

	// if the build plan also has "inputs" field, patch it too
	if inputs, ok := buildPlan["inputs"].([]interface{}); ok {
		var patchedInputs []interface{}
		for _, input := range inputs {
			if inputStr, ok := input.(string); ok {
				for old, new := range revReplacements {
					inputStr = strings.ReplaceAll(inputStr, old, new)
				}
				patchedInputs = append(patchedInputs, inputStr)
			} else {
				patchedInputs = append(patchedInputs, input)
			}
		}
		buildPlan["inputs"] = patchedInputs
	}

	buildPlan["invocations"] = patchedInvocations

	patchedData, err := json.MarshalIndent(buildPlan, "", "    ")
	if err != nil {
		log.Fatalf("Failed to marshal patched JSON: %v", err)
	}

	if err := os.WriteFile(buildPlanPath, patchedData, 0644); err != nil {
		log.Fatalf("Failed to write patched file: %v", err)
	}

	log.Printf("Patched build plan saved to %s", buildPlanPath)
}

// run mode executes the commands from the build plan.
func run(buildPlanPath string, replacements map[string]string) {
	cmd := exec.Command(rustcPath, "-vV")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Failed getting rustc version from %s: %v\nOutput:\n%s", rustcPath, err, string(out))
	} else {
		log.Printf("{{RUSTC}} version: %s", strings.Split(string(out), "\n")[0])
	}

	data, err := os.ReadFile(buildPlanPath)
	if err != nil {
		log.Fatalf("Failed to read build plan file: %v", err)
	}

	var buildPlan map[string]json.RawMessage
	if err := json.Unmarshal(data, &buildPlan); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	var invocationsRaw []json.RawMessage
	if err := json.Unmarshal(buildPlan["invocations"], &invocationsRaw); err != nil {
		log.Fatalf("Failed to unmarshal invocations: %v", err)
	}

	var invocations []Invocation
	for i, invRaw := range invocationsRaw {
		var inv map[string]interface{}
		if err := json.Unmarshal(invRaw, &inv); err != nil {
			log.Fatalf("Failed to unmarshal invocation: %v", err)
		}
		
		replacedInv := deepReplace(inv, replacements).(map[string]interface{})
		replacedJSON, _ := json.Marshal(replacedInv)
		
		var finalInv Invocation
		if err := json.Unmarshal(replacedJSON, &finalInv); err != nil {
			log.Fatalf("Failed to re-unmarshal invocation after replacement: %v", err)
		}
		finalInv.Number = i
		invocations = append(invocations, finalInv)
	}

	sort.SliceStable(invocations, func(i, j int) bool {
		return invocations[i].Number < invocations[j].Number
	})

	invocations = resolveInvocationOrder(invocations)

	// Create target directories
	for _, inv := range invocations {
		for _, output := range inv.Outputs {
			dir := filepath.Dir(output)
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Fatalf("Failed to create directory %s: %v", dir, err)
			}
		}
	}

	customBuildDirectives := make(map[string]*CustomBuildDirectives)

	for i, inv := range invocations {
		cmdArgs := inv.Args
		cmdPath := inv.Program
		if cmdPath == "" {
			cmdPath = rustcPath
		}

		// Apply custom build directives from dependencies
		if d, ok := customBuildDirectives[inv.PackageName]; ok {
			cmdArgs = append(cmdArgs, d.RustcFlags...)
			for k, v := range d.EnvVars {
				inv.Env[k] = v
			}
		}

		// Ensure OUT_DIR exists
		if outDir, ok := inv.Env["OUT_DIR"]; ok {
			if err := os.MkdirAll(outDir, 0755); err != nil {
				log.Fatalf("Failed to create OUT_DIR %s: %v", outDir, err)
			}
		}

		// Prepare command and environment
		cmd := exec.Command(cmdPath, cmdArgs...)
		cmd.Dir = inv.Cwd
		cmd.Env = os.Environ()
		// cmd.Env = append(cmd.Env, "CARGO="+filepath.Join(cargoHome, "bin", "cargo"))
		cmd.Env = append(cmd.Env, fmt.Sprintf("RUSTC=%s", rustcPath))
		cmd.Env = append(cmd.Env, fmt.Sprintf("CARGO_HOME=%s", cargoHome))
		cmd.Env = append(cmd.Env, fmt.Sprintf("PROJECT_ROOT=%s", projectRoot))
		for k, v := range inv.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
		
		log.Printf("(%d/%d) Running '%s' for package '%s' v%s", i, len(invocations), inv.Program, inv.PackageName, inv.PackageVersion)
		args_str := strings.Join(cmdArgs, " ")
		if len(args_str) > 100 {
			args_str = args_str[:100] + "..."
		}
		log.Printf("Invoking: %s %s", cmdPath, args_str)
		// Run the command
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			log.Printf("Command failed:\n%s %s", cmdPath, strings.Join(cmdArgs, " "))
			log.Printf("Command stdout:\n%s", stdout.String())
			log.Printf("Command stderr:\n%s", stderr.String())
			log.Fatalf("Command failed with exit code %v: %v", cmd.ProcessState.ExitCode(), err)
		}

		// Create symlinks
		for link, target := range inv.Links {
			if _, err := os.Lstat(link); err == nil {
				if err := os.Remove(link); err != nil {
					log.Fatalf("Failed to remove existing symlink %s: %v", link, err)
				}
			}
			if err := os.Symlink(target, link); err != nil {
				log.Fatalf("Failed to create symlink %s -> %s: %v", link, target, err)
			}
			log.Printf("Created symlink: %s -> %s", link, target)
		}

		// Capture build script outputs
		if inv.CompileMode == "run-custom-build" {
			customBuildDirectives[inv.PackageName] = NewCustomBuildDirectives(stdout.String())
		}
	}
	log.Println("Build plan execution complete.")
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	
	mode := ""
	buildPlanFile := ""

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("Usage: cargo-go [patch|run] <build-plan.json>")
		os.Exit(1)
	}
	mode = args[0]
	if len(args) > 1 {
		buildPlanFile = args[1]
	}

	var err error
	projectRoot, err = os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}
	if pr, ok := os.LookupEnv("PROJECT_ROOT"); ok {
		projectRoot = pr
	}

	cargoHome = os.Getenv("CARGO_HOME")
	if cargoHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get user home directory: %v", err)
		}
		cargoHome = filepath.Join(homeDir, ".cargo")
	}

	rustcPath, err = findRustc()
	if err != nil {
		log.Fatalf("Failed to find rustc: %v", err)
	}

	log.Printf("PROJECT_ROOT: %s", projectRoot)
	log.Printf("CARGO_HOME: %s", cargoHome)
	log.Printf("RUSTC: %s", rustcPath)

	replacements := map[string]string{
		"{{PROJECT_ROOT}}": projectRoot,
		"{{CARGO_HOME}}":   cargoHome,
		"{{RUSTC}}":        rustcPath,
	}

	switch mode {
	case "patch":
		patch(buildPlanFile, replacements)
	case "run":
		run(buildPlanFile, replacements)
	default:
		log.Fatalf("Unknown mode: %s", mode)
	}
}
