package executor

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/evergreen-ci/utility"
	"github.com/mongodb/jasper/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type executorConstructor func(ctx context.Context, args []string) (Executor, error)

func executorTypes() map[string]executorConstructor {
	return map[string]executorConstructor{
		"StandardExec": func(ctx context.Context, args []string) (Executor, error) {
			return NewLocal(ctx, args), nil
		},
	}
}

type executorTestCase struct {
	Name string
	Case func(ctx context.Context, t *testing.T, makeExec executorConstructor)
}

func executorTestCases() []executorTestCase {
	return []executorTestCase{
		{
			Name: "SetAndGetCommandArgsWorkAsExpected",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				args := []string{"echo", "hello"}
				exec, err := makeExec(ctx, args)
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				assert.Equal(t, args, exec.Args())
			},
		},
		{
			Name: "SetAndGetEnvWorkAsExpected",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				exec, err := makeExec(ctx, []string{"env"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				assert.Empty(t, exec.Env())

				env := []string{"foo=bar", "bat=baz"}
				exec.SetEnv(env)
				assert.Equal(t, env, exec.Env())
				stdout := utility.MakeSafeBuffer(bytes.Buffer{})
				exec.SetStdout(stdout)
				require.NoError(t, exec.Start())
				require.NoError(t, exec.Wait())
				for _, envVar := range env {
					assert.Contains(t, stdout.String(), envVar)
				}
			},
		},
		{
			Name: "SetAndGetWorkingDirWorkAsExpected",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				exec, err := makeExec(ctx, []string{"true"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				assert.Empty(t, exec.Dir())
				dir := "/some/dir"
				exec.SetDir(dir)
				assert.Equal(t, dir, exec.Dir())
			},
		},
		{
			Name: "SetAndGetStdoutWorkAsExpected",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				output := "hello"
				exec, err := makeExec(ctx, []string{"echo", "-n", output})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				stdout := utility.MakeSafeBuffer(bytes.Buffer{})
				exec.SetStdout(stdout)
				require.Equal(t, stdout, exec.Stdout())
				require.NoError(t, exec.Start())
				require.NoError(t, exec.Wait())
				assert.Equal(t, output, stdout.String())
			},
		},
		{
			Name: "SetStdinWorksAsExpected",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				exec, err := makeExec(ctx, []string{"tee"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				input := "hello"
				stdin := bytes.NewBufferString(input)
				exec.SetStdin(stdin)
				stdout := utility.MakeSafeBuffer(bytes.Buffer{})
				exec.SetStdout(stdout)
				require.NoError(t, exec.Start())
				require.NoError(t, exec.Wait())
				assert.Equal(t, input, stdout.String())
			},
		},
		{
			Name: "SetAndGetStderrWorkAsExpected",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				output := "hello"
				exec, err := makeExec(ctx, []string{"bash", "-c", fmt.Sprintf("echo -n %s 1>&2", output)})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				stderr := utility.MakeSafeBuffer(bytes.Buffer{})
				exec.SetStderr(stderr)
				require.Equal(t, stderr, exec.Stderr())
				require.NoError(t, exec.Start())
				require.NoError(t, exec.Wait())
				assert.Equal(t, output, stderr.String())
			},
		},
		{
			Name: "RuntimeFieldsAreInvalidBeforeProcessHasRun",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				exec, err := makeExec(ctx, []string{"true"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				assert.Equal(t, -1, exec.PID())
				assert.False(t, exec.Success())
				assert.Equal(t, -1, exec.ExitCode())
			},
		},
		{
			Name: "StartBeginsProcessExecution",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				exec, err := makeExec(ctx, []string{"sleep", "1"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				require.NoError(t, exec.Start())
				assert.True(t, exec.PID() > 0, "PID '%d' should be positive", exec.PID())
				assert.False(t, exec.Success())
				assert.Equal(t, -1, exec.ExitCode())
			},
		},
		{
			Name: "WaitFailsWithUnstartedProcess",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				exec, err := makeExec(ctx, []string{"true"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				assert.Error(t, exec.Wait())
			},
		},
		{
			Name: "WaitBlocksUntilProcessCompletes",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				exec, err := makeExec(ctx, []string{"sleep", "1"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				require.NoError(t, exec.Start())
				require.NoError(t, exec.Wait())
				assert.True(t, exec.PID() > 0, "PID '%d' should be positive", exec.PID())
				assert.True(t, exec.Success())
				assert.Zero(t, exec.ExitCode())
			},
		},
		{
			Name: "NonZeroExitCodeIsUnsuccessfulProcess",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				exec, err := makeExec(ctx, []string{"false"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				require.NoError(t, exec.Start())
				require.Error(t, exec.Wait())
				assert.False(t, exec.Success())
				assert.True(t, exec.ExitCode() > 0)
			},
		},
		{
			Name: "ContextDoneDoesNotCauseCloseToError",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				cctx, ccancel := context.WithCancel(ctx)
				ccancel()
				exec, err := makeExec(cctx, []string{"true"})
				require.NoError(t, err)
				assert.NoError(t, exec.Close())
			},
		},
		{
			Name: "StartFailsWhenContextIsDone",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				cctx, ccancel := context.WithCancel(ctx)
				defer ccancel()
				exec, err := makeExec(cctx, []string{"true"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				ccancel()
				assert.Error(t, exec.Start())
			},
		},
		{
			Name: "WaitFailsWhenContextIsDoneBeforeProcessExits",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				cctx, ccancel := context.WithCancel(ctx)
				defer ccancel()
				exec, err := makeExec(cctx, []string{"sleep", "1"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				require.NoError(t, exec.Start())
				ccancel()
				assert.Error(t, exec.Wait())
			},
		},
		{
			Name: "ProcessIsUnsignaledByDefault",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				exec, err := makeExec(ctx, []string{"true"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				sig, signaled := exec.SignalInfo()
				assert.False(t, signaled)
				assert.EqualValues(t, -1, sig)
			},
		},
		{
			Name: "SignallingProcessPopulatesSignalInfo",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				if runtime.GOOS == "windows" {
					t.Skip("The standard library implementation of exec does not support signal detection.")
				}
				exec, err := makeExec(ctx, []string{"sleep", "1"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				expected := syscall.SIGKILL
				require.NoError(t, exec.Start())
				require.NoError(t, exec.Signal(expected))
				assert.Error(t, exec.Wait())
				sig, signaled := exec.SignalInfo()
				assert.True(t, signaled)
				assert.Equal(t, expected, sig)
			},
		},
		{
			Name: "SIGKILLedProcessIsUnsuccessful",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				exec, err := makeExec(ctx, []string{"sleep", "1"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				require.NoError(t, exec.Start())
				require.NoError(t, exec.Signal(syscall.SIGKILL))
				require.Error(t, exec.Wait())
				assert.False(t, exec.Success())
				assert.NotZero(t, exec.ExitCode())
			},
		},
		{
			Name: "ProcessThatExitsDueToContextCancellationIsTreatedAsSIGKILLed",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				if runtime.GOOS == "windows" {
					t.Skip("The standard library implementation of exec does not support signal detection.")
				}
				cctx, ccancel := context.WithCancel(ctx)
				defer ccancel()
				exec, err := makeExec(cctx, []string{"sleep", "1"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				require.NoError(t, exec.Start())
				ccancel()
				assert.Error(t, exec.Wait())
				sig, signaled := exec.SignalInfo()
				assert.True(t, signaled)
				assert.Equal(t, syscall.SIGKILL, sig)
				assert.False(t, exec.Success())
				assert.Equal(t, -1, exec.ExitCode())
			},
		},
		{
			Name: "ProcessThatExitsDueToContextTimeoutIsTreatedAsSIGKILLed",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				if runtime.GOOS == "windows" {
					t.Skip("The standard library implementation of exec does not support signal detection.")
				}
				tctx, tcancel := context.WithTimeout(ctx, 2*time.Second)
				defer tcancel()
				exec, err := makeExec(tctx, []string{"sleep", "10"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				require.NoError(t, exec.Start())
				assert.Error(t, exec.Wait())
				sig, signaled := exec.SignalInfo()
				assert.True(t, signaled)
				assert.Equal(t, syscall.SIGKILL, sig)
				assert.False(t, exec.Success())
				assert.Equal(t, -1, exec.ExitCode())
			},
		},
		{
			Name: "ProcessCannotBeSignaledAfterCompletion",
			Case: func(ctx context.Context, t *testing.T, makeExec executorConstructor) {
				exec, err := makeExec(ctx, []string{"true"})
				require.NoError(t, err)
				defer func() {
					assert.NoError(t, exec.Close())
				}()
				require.NoError(t, exec.Start())
				require.NoError(t, exec.Wait())
				assert.Error(t, exec.Signal(syscall.SIGKILL))
				sig, signaled := exec.SignalInfo()
				assert.False(t, signaled)
				assert.EqualValues(t, -1, sig)
			},
		},
	}
}

func TestExecutor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for execType, makeExec := range executorTypes() {
		t.Run(execType, func(t *testing.T) {
			for _, testCase := range executorTestCases() {
				t.Run(testCase.Name, func(t *testing.T) {
					tctx, tcancel := context.WithTimeout(ctx, testutil.ExecutorTestTimeout)
					defer tcancel()
					testCase.Case(tctx, t, makeExec)
				})
			}
		})
	}
}
