package woodsman

import (
    "bytes"
    "fmt"
    "path/filepath"
    "runtime"
    "strings"
    "testing"
    "time"
    "os"
    "io/ioutil"
    "syscall"
)

// Test that shortHostname works as advertised.
func TestShortHostname(t *testing.T) {
    for hostname, expect := range map[string]string{
        "":                "",
        "host":            "host",
        "host.google.com": "host",
    } {
        if got := shortHostname(hostname); expect != got {
            t.Errorf("shortHostname(%q): expected %q, got %q", hostname, expect, got)
        }
    }
}

// flushBuffer wraps a bytes.Buffer to satisfy flushSyncWriter.
type flushBuffer struct {
    bytes.Buffer
}

func (f *flushBuffer) Flush() error {
    return nil
}

func (f *flushBuffer) Sync() error {
    return nil
}

// swap sets the log writers and returns the old array.
func (l *loggingT) swap(writers [numSeverity]flushSyncWriter) (old [numSeverity]flushSyncWriter) {
    l.mu.Lock()
    defer l.mu.Unlock()
    old = l.file
    for i, w := range writers {
        logging.file[i] = w
    }
    return
}

// newBuffers sets the log writers to all new byte buffers and returns the old array.
func (l *loggingT) newBuffers() [numSeverity]flushSyncWriter {
    return l.swap([numSeverity]flushSyncWriter{new(flushBuffer), new(flushBuffer), new(flushBuffer), new(flushBuffer)})
}

// contents returns the specified log value as a string.
func contents(s severity) string {
    return logging.file[s].(*flushBuffer).String()
}

// contains reports whether the string is contained in the log.
func contains(s severity, str string, t *testing.T) bool {
    return strings.Contains(contents(s), str)
}

// setFlags configures the logging flags how the test expects them.
func setFlags() {
    logging.toStderr = false
    logging.toSyslog = false
    logging.toFile = true
}

func TestInitVariables(t *testing.T) {
    logToFile := os.Getenv("WOODSMAN_LOGTOFILE")
    os.Setenv("WOODSMAN_LOGTOFILE", "TRUE")
    tmpdir := os.Getenv("TMPDIR")
    os.Setenv("TMPDIR", "/tmp/woodsman")
    syscall.Mkdir("/tmp/woodsman", 0777)

    logMsg := "THIS SHOULD BE WRITTEN TO A FILE"

    Info(logMsg)

    // read whole the file
    b, err := ioutil.ReadFile("/tmp/woodsman/main.test.INFO")
    if err != nil {
        t.Errorf("Expected no errors when opening log file - got %v", err)   
    }

    if !strings.Contains(string(b), logMsg) {
        t.Errorf("Expected file to contain %v - got %v.\n", logMsg, string(b))
    }

    os.Setenv("WOODSMAN_LOGTOFILE", logToFile)
    os.Setenv("TMPDIR", tmpdir)
}

// If toFile is false and logDir is set, we should get a true bool to alert user log_dir is being ignored
func TestFlagSetLogicWarnings(t *testing.T) {
    if getLogDirWarning(true, "") {
        t.Errorf("getLogDirWarning logic is incorrect - got %v, expected %v", getLogDirWarning(true, ""), false)
    }

    if getLogDirWarning(false, "") {
        t.Errorf("getLogDirWarning logic is incorrect - got %v, expected %v", getLogDirWarning(false, ""), false)
    }

    if getLogDirWarning(true, "/some/directory") {
        t.Errorf("getLogDirWarning logic is incorrect - got %v, expected %v", getLogDirWarning(true, "/some/directory"), false)
    }

    if !getLogDirWarning(false, "/some/directory") {
        t.Errorf("getLogDirWarning logic is incorrect - got %v, expected %v", getLogDirWarning(false, "/some/directory"), true)
    }
}

// Test that the header has the correct format.
func TestHeader(t *testing.T) {
    setFlags()
    defer logging.swap(logging.newBuffers())
    defer func(previous func() time.Time) { timeNow = previous }(timeNow)
    timeNow = func() time.Time {
        return time.Date(2006, 1, 2, 15, 4, 5, .678901e9, time.Local)
    }
    Info("test")
    var line, pid int
    n, err := fmt.Sscanf(contents(infoLog), "I0102 15:04:05.678901 %d woodsman_test.go:%d] test\n", &pid, &line)
    if n != 2 || err != nil {
        t.Errorf("log format error: %d elements, error %s:\n%s", n, err, contents(infoLog))
    }
}

// Test that Info works as advertised.
func TestInfo(t *testing.T) {
    setFlags()
    defer logging.swap(logging.newBuffers())
    Info("test")
    if !contains(infoLog, "I", t) {
        t.Errorf("Info has wrong character: %q", contents(infoLog))
    }
    if !contains(infoLog, "test", t) {
        t.Error("Info failed")
    }
}

// Test that an Error log goes to Warning and Info.
// Even in the Info log, the source character will be E, so the data should
// all be identical.
func TestError(t *testing.T) {
    setFlags()
    defer logging.swap(logging.newBuffers())
    Error("test")
    if !contains(errorLog, "E", t) {
        t.Errorf("Error has wrong character: %q", contents(errorLog))
    }
    if !contains(errorLog, "test", t) {
        t.Error("Error failed")
    }
}

// Test that a Warning log goes to Info.
// Even in the Info log, the source character will be W, so the data should
// all be identical.
func TestWarning(t *testing.T) {
    setFlags()
    defer logging.swap(logging.newBuffers())
    Warning("test")
    if !contains(warningLog, "W", t) {
        t.Errorf("Warning has wrong character: %q", contents(warningLog))
    }
    if !contains(warningLog, "test", t) {
        t.Error("Warning failed")
    }
}

// Test that a V log goes to Info.
func TestV(t *testing.T) {
    setFlags()
    defer logging.swap(logging.newBuffers())
    logging.verbosity.Set("2")
    defer logging.verbosity.Set("0")
    V(2).Info("test")
    if !contains(infoLog, "I", t) {
        t.Errorf("Info has wrong character: %q", contents(infoLog))
    }
    if !contains(infoLog, "test", t) {
        t.Error("Info failed")
    }
}

// Test that a vmodule enables a log in this file.
func TestVmoduleOn(t *testing.T) {
    setFlags()
    defer logging.swap(logging.newBuffers())
    logging.vmodule.Set("woodsman_test=2")
    defer logging.vmodule.Set("")
    if !V(1) {
        t.Error("V not enabled for 1")
    }
    if !V(2) {
        t.Error("V not enabled for 2")
    }
    if V(3) {
        t.Error("V enabled for 3")
    }
    V(2).Info("test")
    if !contains(infoLog, "I", t) {
        t.Errorf("Info has wrong character: %q", contents(infoLog))
    }
    if !contains(infoLog, "test", t) {
        t.Error("Info failed")
    }
}

// Test that a vmodule of another file does not enable a log in this file.
func TestVmoduleOff(t *testing.T) {
    setFlags()
    defer logging.swap(logging.newBuffers())
    logging.vmodule.Set("notthisfile=2")
    defer logging.vmodule.Set("")
    for i := 1; i <= 3; i++ {
        if V(Level(i)) {
            t.Errorf("V enabled for %d", i)
        }
    }
    V(2).Info("test")
    if contents(infoLog) != "" {
        t.Error("V logged incorrectly")
    }
}

// vGlobs are patterns that match/don't match this file at V=2.
var vGlobs = map[string]bool{
    // Easy to test the numeric match here.
    "woodsman_test=1": false, // If -vmodule sets V to 1, V(2) will fail.
    "woodsman_test=2": true,
    "woodsman_test=3": true, // If -vmodule sets V to 1, V(3) will succeed.
    // These all use 2 and check the patterns. All are true.
    "*=2":   true,
    "?o*=2": true,
    // These all use 2 and check the patterns. All are false.
    "*x=2":         false,
    "m*=2":         false,
    "??_*=2":       false,
    "?[abc]?_*t=2": false,
}

// Test that vmodule globbing works as advertised.
func testVmoduleGlob(pat string, match bool, t *testing.T) {
    setFlags()
    defer logging.swap(logging.newBuffers())
    defer logging.vmodule.Set("")
    logging.vmodule.Set(pat)
    if V(2) != Verbose(match) {
        t.Errorf("incorrect match for %q: got %t expected %t", pat, V(2), match)
    }
}

// Test that a vmodule globbing works as advertised.
func TestVmoduleGlob(t *testing.T) {
    for glob, match := range vGlobs {
        testVmoduleGlob(glob, match, t)
    }
}

func TestRollover(t *testing.T) {
    setFlags()
    var err error
    defer func(previous func(error)) { logExitFunc = previous }(logExitFunc)
    logExitFunc = func(e error) {
        err = e
    }
    defer func(previous uint64) { MaxSize = previous }(MaxSize)
    MaxSize = 512

    Info("x") // Be sure we have a file.
    info, ok := logging.file[infoLog].(*syncBuffer)
    if !ok {
        t.Fatal("info wasn't created")
    }
    if err != nil {
        t.Fatalf("info has initial error: %v", err)
    }
    fname0 := info.file.Name()
    Info(strings.Repeat("x", int(MaxSize))) // force a rollover
    if err != nil {
        t.Fatalf("info has error after big write: %v", err)
    }

    // Make sure the next log file gets a file name with a different
    // time stamp.
    //
    // TODO: determine whether we need to support subsecond log
    // rotation.  C++ does not appear to handle this case (nor does it
    // handle Daylight Savings Time properly).
    time.Sleep(1 * time.Second)

    Info("x") // create a new file
    if err != nil {
        t.Fatalf("error after rotation: %v", err)
    }
    fname1 := info.file.Name()
    if fname0 == fname1 {
        t.Errorf("info.f.Name did not change: %v", fname0)
    }
    if info.nbytes >= MaxSize {
        t.Errorf("file size was not reset: %d", info.nbytes)
    }
}

func TestLogBacktraceAt(t *testing.T) {
    setFlags()
    defer logging.swap(logging.newBuffers())
    // The peculiar style of this code simplifies line counting and maintenance of the
    // tracing block below.
    var infoLine string
    setTraceLocation := func(file string, line int, ok bool, delta int) {
        if !ok {
            t.Fatal("could not get file:line")
        }
        _, file = filepath.Split(file)
        infoLine = fmt.Sprintf("%s:%d", file, line+delta)
        err := logging.traceLocation.Set(infoLine)
        if err != nil {
            t.Fatal("error setting log_backtrace_at: ", err)
        }
    }
    {
        // Start of tracing block. These lines know about each other's relative position.
        _, file, line, ok := runtime.Caller(0)
        setTraceLocation(file, line, ok, +2) // Two lines between Caller and Info calls.
        Info("we want a stack trace here")
    }
    numAppearances := strings.Count(contents(infoLog), infoLine)
    if numAppearances < 2 {
        t.Fatal("got no trace back; log is ", contents(infoLog))
    }
}

func BenchmarkHeader(b *testing.B) {
    for i := 0; i < b.N; i++ {
        logging.putBuffer(logging.header(infoLog))
    }
}
