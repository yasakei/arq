package runtime

import (
	"github.com/yasakei/arq/internal/ast"
	"os"
	"path/filepath"
	"time"
)

func (r *Runner) startWatch(pattern string, body ast.Block, env *Env) {
	stop := make(chan struct{})
	r.mu.Lock()
	r.watchers = append(r.watchers, stop)
	r.mu.Unlock()
	go func() {
		files := map[string]time.Time{}
		snapshot := func() {
			matches, _ := filepath.Glob(filepath.Join(r.Root, pattern))
			for _, path := range matches {
				if stat, err := fileStat(path); err == nil {
					files[path] = stat
				}
			}
		}
		snapshot()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				changed := false
				matches, _ := filepath.Glob(filepath.Join(r.Root, pattern))
				seen := map[string]bool{}
				for _, path := range matches {
					seen[path] = true
					stamp, err := fileStat(path)
					if err == nil && (!files[path].Equal(stamp)) {
						changed = true
						files[path] = stamp
					}
				}
				for path := range files {
					if !seen[path] {
						changed = true
						delete(files, path)
					}
				}
				if changed {
					if err := r.runBlock(body, newEnv(env)); err != nil {
						r.Err.WriteString("arq: watch: " + err.Error() + "\n")
					}
				}
			}
		}
	}()
}

func fileStat(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func (r *Runner) stopWatches() {
	r.mu.Lock()
	watchers := r.watchers
	r.watchers = nil
	r.mu.Unlock()
	for _, stop := range watchers {
		close(stop)
	}
}
