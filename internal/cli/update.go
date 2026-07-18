package cli

import (
	"context"
	"errors"
	"fmt"
)

func (a *App) runUpdate(ctx context.Context, args []string) int {
	if a.Updater == nil {
		fmt.Fprintln(a.IO.Err, "当前构建不支持自更新")
		return ExitInternal
	}
	if len(args) == 1 && args[0] == "check" {
		current, latest, available, err := a.Updater.Check(ctx)
		if err != nil {
			fmt.Fprintf(a.IO.Err, "检查更新失败: %v\n", err)
			if errors.Is(err, context.Canceled) {
				return ExitCancelled
			}
			return ExitExtraction
		}
		fmt.Fprintf(a.IO.Out, "当前版本: %s\n最新版本: %s\n", current, latest)
		if available {
			fmt.Fprintln(a.IO.Out, "有可用更新；运行 music2bb update 安装。")
		} else {
			fmt.Fprintln(a.IO.Out, "已是最新版本。")
		}
		return ExitSuccess
	}
	if len(args) != 0 {
		fmt.Fprintln(a.IO.Err, "用法: music2bb update [check]")
		return ExitInvalidInput
	}
	from, to, deferred, err := a.Updater.Update(ctx)
	if err != nil {
		fmt.Fprintf(a.IO.Err, "更新失败: %v\n", err)
		if errors.Is(err, context.Canceled) {
			return ExitCancelled
		}
		return ExitExtraction
	}
	if from == to {
		fmt.Fprintf(a.IO.Out, "已是最新版本: %s\n", to)
		return ExitSuccess
	}
	if deferred {
		fmt.Fprintf(a.IO.Out, "已下载 %s；当前进程退出后将完成替换。\n", to)
		return ExitSuccess
	}
	fmt.Fprintf(a.IO.Out, "已从 %s 更新到 %s。\n", from, to)
	return ExitSuccess
}
