package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type cachePaths struct {
	search            string
	checkpoints       string
	decisions         string
	anonymousIdentity string
}

type cachePathStatus struct {
	files int
	bytes int64
}

func (a *App) runCache(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(a.IO.Err, "用法: music2bb cache status|clear")
		return ExitInvalidInput
	}
	paths, err := a.cachePaths()
	if err != nil {
		fmt.Fprintf(a.IO.Err, "缓存路径读取失败: %v\n", err)
		return ExitInternal
	}
	switch args[0] {
	case "status":
		return a.runCacheStatus(args[1:], paths)
	case "clear":
		return a.runCacheClear(ctx, args[1:], paths)
	default:
		fmt.Fprintf(a.IO.Err, "未知 cache 子命令: %s\n", args[0])
		return ExitInvalidInput
	}
}

func (a *App) runCacheStatus(args []string, paths cachePaths) int {
	set := newFlagSet("cache status", a.IO.Err)
	var configDir string
	set.StringVar(&configDir, "config-dir", "", "配置目录")
	if err := set.Parse(interspersed(args, map[string]bool{"--config-dir": true})); err != nil {
		if err == flag.ErrHelp {
			return ExitSuccess
		}
		return ExitInvalidInput
	}
	if set.NArg() != 0 {
		fmt.Fprintln(a.IO.Err, "用法: music2bb cache status")
		return ExitInvalidInput
	}
	for _, item := range []struct {
		name string
		path string
	}{
		{name: "search", path: paths.search},
		{name: "checkpoints", path: paths.checkpoints},
		{name: "decisions", path: paths.decisions},
		{name: "anonymous-identity", path: paths.anonymousIdentity},
	} {
		status, err := inspectCachePath(item.path)
		if err != nil {
			fmt.Fprintf(a.IO.Err, "读取 %s 状态失败: %v\n", item.name, err)
			return ExitInternal
		}
		fmt.Fprintf(a.IO.Out, "%s\tfiles=%d\tbytes=%d\n", item.name, status.files, status.bytes)
	}
	return ExitSuccess
}

func (a *App) runCacheClear(ctx context.Context, args []string, paths cachePaths) int {
	set := newFlagSet("cache clear", a.IO.Err)
	var search, checkpoints, decisions, anonymousIdentity, all bool
	var configDir string
	set.BoolVar(&search, "search", false, "清除搜索缓存")
	set.BoolVar(&checkpoints, "checkpoints", false, "清除转换 checkpoint")
	set.BoolVar(&decisions, "decisions", false, "清除历史人工决定")
	set.BoolVar(&anonymousIdentity, "anonymous-identity", false, "清除匿名设备状态")
	set.BoolVar(&all, "all", false, "清除以上全部状态")
	set.StringVar(&configDir, "config-dir", "", "配置目录")
	if err := set.Parse(interspersed(args, map[string]bool{"--config-dir": true})); err != nil {
		if err == flag.ErrHelp {
			return ExitSuccess
		}
		return ExitInvalidInput
	}
	if set.NArg() != 0 || (!search && !checkpoints && !decisions && !anonymousIdentity && !all) {
		fmt.Fprintln(a.IO.Err, "用法: music2bb cache clear --search|--checkpoints|--decisions|--anonymous-identity|--all")
		return ExitInvalidInput
	}
	if all {
		search, checkpoints, decisions, anonymousIdentity = true, true, true, true
	}
	for _, item := range []struct {
		selected bool
		name     string
		path     string
	}{
		{selected: search, name: "search", path: paths.search},
		{selected: checkpoints, name: "checkpoints", path: paths.checkpoints},
		{selected: decisions, name: "decisions", path: paths.decisions},
	} {
		if !item.selected {
			continue
		}
		if err := os.RemoveAll(item.path); err != nil {
			fmt.Fprintf(a.IO.Err, "清除 %s 失败: %v\n", item.name, err)
			return ExitInternal
		}
		fmt.Fprintf(a.IO.Out, "cleared\t%s\n", item.name)
	}
	if anonymousIdentity {
		resetter, ok := a.Backend.(interface{ ResetAnonymousIdentity(context.Context) error })
		if !ok {
			fmt.Fprintln(a.IO.Err, "清除 anonymous-identity 失败: 后端不支持匿名设备状态重置")
			return ExitInternal
		}
		if err := resetter.ResetAnonymousIdentity(ctx); err != nil {
			fmt.Fprintf(a.IO.Err, "清除 anonymous-identity 失败: %v\n", err)
			return exitFor(err)
		}
		if err := os.Remove(paths.anonymousIdentity); err != nil && !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(a.IO.Err, "清除 anonymous-identity 失败: %v\n", err)
			return ExitInternal
		}
		fmt.Fprintln(a.IO.Out, "cleared\tanonymous-identity")
	}
	return ExitSuccess
}

func (a *App) cachePaths() (cachePaths, error) {
	provider, ok := a.Backend.(interface{ PersistentStatePaths() (string, string) })
	if !ok {
		return cachePaths{}, errors.New("backend does not expose persistent state paths")
	}
	configDir, cacheDir := provider.PersistentStatePaths()
	if configDir == "" || cacheDir == "" {
		return cachePaths{}, errors.New("persistent state paths are empty")
	}
	return cachePaths{
		search:            filepath.Join(cacheDir, "search", "v1"),
		checkpoints:       filepath.Join(configDir, "conversions", "v1"),
		decisions:         filepath.Join(configDir, "decisions", "v1"),
		anonymousIdentity: filepath.Join(configDir, "cookies", "bilibili-anonymous.json"),
	}, nil
}

func inspectCachePath(path string) (cachePathStatus, error) {
	status := cachePathStatus{}
	err := filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			status.files++
			status.bytes += info.Size()
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return cachePathStatus{}, nil
	}
	return status, err
}
