// Package cmd implements the spawn-flowise CLI commands.
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/spawn-flowise/spawn-flowise/internal/config"
	"github.com/spawn-flowise/spawn-flowise/internal/container"
	"github.com/spawn-flowise/spawn-flowise/internal/lock"
	"github.com/spawn-flowise/spawn-flowise/internal/system"
	"github.com/spawn-flowise/spawn-flowise/internal/utils"
)

const lockFileName = ".flowise-spawn.lock"

func runtimeLock() (*lock.Lock, func(), error) {
	dirs, err := config.Dirs()
	if err != nil {
		return nil, nil, err
	}
	return lock.Acquire(filepath.Join(dirs.StateDir, lockFileName))
}

func baseComposePath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	p := filepath.Join(cwd, "docker-compose.yml")
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("docker-compose.yml not found in current directory: %w", err)
	}
	return p, nil
}

func composeFileFor(info config.InstanceInfo) string {
	dirs, _ := config.Dirs()
	return filepath.Join(dirs.StateDir, "compose", info.ServiceName+".yml")
}

// ensureComposeFile generates a per-instance compose file from the base template.
func ensureComposeFile(basePath string, info config.InstanceInfo) error {
	data, err := os.ReadFile(basePath)
	if err != nil {
		return fmt.Errorf("reading base compose template: %w", err)
	}
	s := string(data)
	// Replace the base service name with the instance service name.
	serviceRe := regexp.MustCompile(`(?m)^    flowise:$`)
	s = serviceRe.ReplaceAllString(s, "    "+info.ServiceName+":")
	// Ensure host port binding is on 0.0.0.0 for private-network compatibility.
	portRe := regexp.MustCompile(`(?m)^(\s+- )'\$\{HOST_PORT:-\$\{PORT\}\}:\$\{PORT\}'`)
	s = portRe.ReplaceAllString(s, "${1}'0.0.0.0:$${HOST_PORT:-$${PORT}}:$${PORT}'")

	out := composeFileFor(info)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("creating compose directory: %w", err)
	}
	if err := os.WriteFile(out, []byte(s), 0o644); err != nil {
		return fmt.Errorf("writing compose file: %w", err)
	}
	return nil
}

func ensureDataDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// RunCheck validates engine reachability and host resources.
func RunCheck(engine *container.Engine) error {
	fmt.Printf("Checking %s engine reachability...\n", engine.Type)
	if err := engine.Check(context.Background()); err != nil {
		return err
	}
	fmt.Println("Engine reachable.")

	ram, err := system.TotalRAM()
	if err != nil {
		return fmt.Errorf("cannot determine total RAM: %w", err)
	}
	fmt.Printf("Total RAM: %.2f GB\n", float64(ram)/(1024*1024*1024))

	fmt.Printf("Checking base port %d availability...\n", config.BasePort)
	if !system.IsPortAvailable(config.BasePort) {
		return fmt.Errorf("base port %d is already in use", config.BasePort)
	}
	fmt.Println("Base port available.")
	return nil
}

// RunSpawn creates and starts N isolated Flowise instances sequentially.
func RunSpawn(engine *container.Engine, count int) error {
	lk, release, err := runtimeLock()
	if err != nil {
		return err
	}
	defer release()
	_ = lk

	baseCompose, err := baseComposePath()
	if err != nil {
		return err
	}

	ram, err := system.TotalRAM()
	if err != nil {
		return fmt.Errorf("cannot determine total RAM: %w", err)
	}
	memPerInst := uint64(config.SpawnMemoryCheckMB)
	required := memPerInst * uint64(count) * 1024 * 1024
	if required > ram {
		return fmt.Errorf("insufficient RAM: %d instances need ~%d MB, host has %.0f MB",
			count, memPerInst*uint64(count), float64(ram)/(1024*1024))
	}

	for i := 0; i < count; i++ {
		info, err := config.NewInstanceInfo(i)
		if err != nil {
			return err
		}
		fmt.Printf("\nSpawning instance %s (port %d, network %s)...\n",
			info.ServiceName, info.HostPort, info.NetworkName)

		if !system.IsPortAvailable(info.HostPort) {
			return fmt.Errorf("host port %d is already in use", info.HostPort)
		}

		if err := ensureDataDir(info.DataDir); err != nil {
			return fmt.Errorf("creating data dir %s: %w", info.DataDir, err)
		}
		if err := engine.NetworkCreate(context.Background(), info.NetworkName); err != nil {
			return fmt.Errorf("creating network %s: %w", info.NetworkName, err)
		}
		if err := info.WriteEnvFile(); err != nil {
			return err
		}
		if err := ensureComposeFile(baseCompose, info); err != nil {
			return err
		}

		if err := engine.ComposeUp(context.Background(), info.ServiceName, composeFileFor(info), info.EnvFile); err != nil {
			return err
		}
		fmt.Printf("Instance %s started.\n", info.ServiceName)

		if i < count-1 {
			fmt.Printf("Waiting %ds before next instance...\n", config.SpawnDelaySec)
			time.Sleep(time.Duration(config.SpawnDelaySec) * time.Second)
		}
	}

	fmt.Printf("Waiting %ds for all instances to stabilize...\n", config.SpawnDelaySec)
	time.Sleep(time.Duration(config.SpawnDelaySec) * time.Second)

	fmt.Println("\nSpawn complete.")
	return nil
}

// RunStop stops and removes all flowise-instance-NN containers and their networks.
func RunStop(engine *container.Engine) error {
	lk, release, err := runtimeLock()
	if err != nil {
		return err
	}
	defer release()
	_ = lk

	containers, err := engine.ListProjectContainers(context.Background(), "flowise-instance-")
	if err != nil {
		return err
	}
	for _, name := range containers {
		fmt.Printf("Stopping %s...\n", name)
		if err := engine.StopContainer(context.Background(), name); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to stop %s: %v\n", name, err)
		}
	}
	for _, name := range containers {
		fmt.Printf("Removing container %s...\n", name)
		if err := engine.RemoveContainer(context.Background(), name); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove %s: %v\n", name, err)
		}
	}

	networks, err := engine.ListNetworks(context.Background(), "flowise-default-")
	if err != nil {
		return err
	}
	for _, name := range networks {
		fmt.Printf("Removing network %s...\n", name)
		if err := engine.NetworkRemove(context.Background(), name); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove network %s: %v\n", name, err)
		}
	}

	fmt.Println("Stop complete.")
	return nil
}

// RunHold stops instances and moves their data directories to ~/.bkpflowiseNN.
func RunHold(engine *container.Engine) error {
	lk, release, err := runtimeLock()
	if err != nil {
		return err
	}
	defer release()
	_ = lk

	if err := RunStop(engine); err != nil {
		return err
	}

	dirs, err := config.Dirs()
	if err != nil {
		return err
	}
	instances, err := utils.ListInstanceDirs(dirs.Home)
	if err != nil {
		return err
	}
	for _, inst := range instances {
		if inst.Held {
			continue
		}
		info, err := config.NewInstanceInfo(inst.Number)
		if err != nil {
			return err
		}
		fmt.Printf("Holding instance %02d (%s -> %s)...\n", inst.Number, info.DataDir, info.BackupDataDir)
		if err := utils.SecureRename(info.DataDir, info.BackupDataDir); err != nil {
			return err
		}
	}
	fmt.Println("Hold complete.")
	return nil
}

// RunUnhold restores held data directories from ~/.bkpflowiseNN to ~/.flowiseNN.
func RunUnhold(engine *container.Engine) error {
	lk, release, err := runtimeLock()
	if err != nil {
		return err
	}
	defer release()
	_ = lk

	dirs, err := config.Dirs()
	if err != nil {
		return err
	}
	instances, err := utils.ListInstanceDirs(dirs.Home)
	if err != nil {
		return err
	}
	for _, inst := range instances {
		if !inst.Held {
			continue
		}
		info, err := config.NewInstanceInfo(inst.Number)
		if err != nil {
			return err
		}
		fmt.Printf("Unholding instance %02d (%s -> %s)...\n", inst.Number, info.BackupDataDir, info.DataDir)
		if err := utils.SecureRename(info.BackupDataDir, info.DataDir); err != nil {
			return err
		}
	}
	fmt.Println("Unhold complete.")
	return nil
}

// RunCleanup archives data directories and removes spawned resources.
func RunCleanup(engine *container.Engine) error {
	lk, release, err := runtimeLock()
	if err != nil {
		return err
	}
	defer release()
	_ = lk

	dirs, err := config.Dirs()
	if err != nil {
		return err
	}

	containers, err := engine.ListProjectContainers(context.Background(), "flowise-instance-")
	if err != nil {
		return err
	}
	for _, name := range containers {
		fmt.Printf("Removing container %s...\n", name)
		if err := engine.RemoveContainer(context.Background(), name); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove %s: %v\n", name, err)
		}
	}

	networks, err := engine.ListNetworks(context.Background(), "flowise-default-")
	if err != nil {
		return err
	}
	for _, name := range networks {
		fmt.Printf("Removing network %s...\n", name)
		if err := engine.NetworkRemove(context.Background(), name); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove network %s: %v\n", name, err)
		}
	}

	instances, err := utils.ListInstanceDirs(dirs.Home)
	if err != nil {
		return err
	}

	var activeDirs []string
	for _, inst := range instances {
		if inst.Held {
			// Held data (~/.bkpflowiseNN) is intentionally preserved for unhold.
			continue
		}
		info, err := config.NewInstanceInfo(inst.Number)
		if err != nil {
			return err
		}
		activeDirs = append(activeDirs, info.DataDir)
	}

	if len(activeDirs) > 0 {
		ts := time.Now().UTC().Format("20060102-150405")
		backupPath := filepath.Join(dirs.BackupDir, "flowise_backup_"+ts+".tar.gz")
		fmt.Printf("Archiving %d instance dir(s) to %s...\n", len(activeDirs), backupPath)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		err := utils.ArchiveToTarGz(ctx, activeDirs, backupPath)
		cancel()
		if err != nil {
			return fmt.Errorf("creating backup archive: %w", err)
		}
		fmt.Printf("Archived to %s\n", backupPath)
	}

	if err := os.RemoveAll(dirs.StateDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to remove state dir %s: %v\n", dirs.StateDir, err)
	}

	// Remove active data directories with sudo rm -rf as the final cleanup step.
	for _, d := range activeDirs {
		fmt.Printf("Removing %s...\n", d)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		if err := utils.SudoRemove(ctx, d); err != nil {
			cancel()
			return err
		}
		cancel()
	}

	fmt.Println("Cleanup complete.")
	return nil
}
