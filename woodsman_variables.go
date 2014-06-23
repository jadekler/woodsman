package woodsman

import (
    "flag"
    "fmt"
    "sync"
)

var logging loggingT

// loggingT collects all the global state of the logging setup.
type loggingT struct {
    // Boolean flags. Not handled atomically because the flag.Value interface
    // does not let us avoid the =true, and that shorthand is necessary for
    // compatibility.
    toStderr bool // The -logtostderr flag.
    toFile   bool // The -logtofile flag.
    toSyslog bool // The -logtosyslog flag.
    logDir   string

    // Level flag. Handled atomically.
    stderrThreshold severity // The -stderrthreshold flag.

    // freeList is a list of byte buffers, maintained under freeListMu.
    freeList *buffer
    // freeListMu maintains the free list. It is separate from the main mutex
    // so buffers can be grabbed and printed to without holding the main lock,
    // for better parallelization.
    freeListMu sync.Mutex

    // mu protects the remaining elements of this structure and is
    // used to synchronize logging.
    mu  sync.Mutex
    // file holds writer for each of the log types.
    file [numSeverity]flushSyncWriter
    // pcs is used in V to avoid an allocation when computing the caller's PC.
    pcs [1]uintptr
    // vmap is a cache of the V Level for each V() call site, identified by PC.
    // It is wiped whenever the vmodule flag changes state.
    vmap map[uintptr]Level
    // filterLength stores the length of the vmodule filter chain. If greater
    // than zero, it means vmodule is enabled. It may be read safely
    // using sync.LoadInt32, but is only modified under mu.
    filterLength int32
    // traceLocation is the state of the -log_backtrace_at flag.
    traceLocation traceLocation
    // These flags are modified only under lock, although verbosity may be fetched
    // safely using atomic.LoadInt32.
    vmodule   moduleSpec // The state of the -vmodule flag.
    verbosity Level      // V logging level, the value of the -v flag/
}

func initFlags() {
    flag.BoolVar(&logging.toStderr, "logtostderr", false, "log to standard error")
    flag.BoolVar(&logging.toFile, "logtofile", false, "log to files")
    flag.BoolVar(&logging.toSyslog, "logtosyslog", false, "log to syslog")
    flag.Var(&logging.verbosity, "v", "log level for V logs")
    flag.Var(&logging.stderrThreshold, "stderrthreshold", "logs at or above this threshold go to stderr")
    flag.Var(&logging.vmodule, "vmodule", "comma-separated list of pattern=N settings for file-filtered logging")
    flag.Var(&logging.traceLocation, "log_backtrace_at", "when logging hits line file:N, emit a stack trace")
    flag.StringVar(&logging.logDir, "log_dir", "", "If non-empty, write log files in this directory")

    if getLogDirWarning(logging.toFile, logging.logDir) {
        fmt.Println("Warning: -log_dir is set, but -logtofile is not. -log_dir will be ignored.")
    }
}

// Return true if logDir is set and toFile is false
func getLogDirWarning(toFile bool, logDir string) bool {
    return logDir != "" && !toFile
}
