package containerrun_test

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/cf-operator/container-run/pkg/containerrun"
	. "code.cloudfoundry.org/cf-operator/container-run/pkg/containerrun/mocks"
)

var _ = Describe("Run", func() {
	commandLine := []string{"bash", "-c", "echo foo"}
	command := Command{
		Name: commandLine[0],
		Arg:  commandLine[1:],
	}
	postStartLine := []string{"bash", "-c", "echo bar"}
	postStart := Command{
		Name: postStartLine[0],
		Arg:  postStartLine[1:],
	}
	postStartConditionLine := []string{"bash", "-c", "echo baz"}
	postStartCondition := Command{
		Name: postStartConditionLine[0],
		Arg:  postStartConditionLine[1:],
	}
	stdio := Stdio{}

	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("fails when args is empty", func() {
		err := Run(nil, nil, nil, stdio, []string{}, "", []string{}, "", []string{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("failed to run container: a command is required"))
	})

	It("fails when runner.Run fails", func() {
		runner := NewMockRunner(ctrl)
		runner.EXPECT().
			Run(command, stdio).
			Return(nil, fmt.Errorf(`¯\_(ツ)_/¯`)).
			Times(1)
		err := Run(runner, nil, nil, stdio, commandLine, "", []string{}, "", []string{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(`failed to run container: ¯\_(ツ)_/¯`))
	})

	It("fails when process.Wait fails", func() {
		process := NewMockProcess(ctrl)
		process.EXPECT().
			Wait().
			Return(fmt.Errorf(`¯\_(ツ)_/¯`)).
			Times(1)
		process.EXPECT().
			Signal(gomock.Any()).
			Return(nil).
			AnyTimes()
		runner := NewMockRunner(ctrl)
		runner.EXPECT().
			Run(command, stdio).
			Return(process, nil).
			Times(1)
		err := Run(runner, nil, nil, stdio, commandLine, "", []string{}, "", []string{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(`failed to run container: ¯\_(ツ)_/¯`))
	})

	It("succeeds when process.Wait succeeds", func() {
		process := NewMockProcess(ctrl)
		process.EXPECT().
			Wait().
			Return(nil).
			Times(1)
		process.EXPECT().
			Signal(gomock.Any()).
			Return(nil).
			AnyTimes()
		runner := NewMockRunner(ctrl)
		runner.EXPECT().
			Run(command, stdio).
			Return(process, nil).
			Times(1)
		err := Run(runner, nil, nil, stdio, commandLine, "", []string{}, "", []string{})
		Expect(err).ToNot(HaveOccurred())
	})

	It("skips post start when the command does not exist", func() {
		process := NewMockProcess(ctrl)
		process.EXPECT().
			Wait().
			Return(nil).
			Times(1)
		process.EXPECT().
			Signal(gomock.Any()).
			Return(nil).
			AnyTimes()
		runner := NewMockRunner(ctrl)
		runner.EXPECT().
			Run(command, stdio).
			Return(process, nil).
			Times(1)
		checker := NewMockChecker(ctrl)
		checker.EXPECT().
			Check(postStart.Name).
			Return(false).
			Times(1)
		err := Run(runner, nil, checker, stdio, commandLine, postStart.Name, postStart.Arg, "", []string{})
		Expect(err).ToNot(HaveOccurred())
	})

	Context("With post-start condition", func() {
		It("fails when post-start RunContext fails", func() {
			expectedErr := fmt.Errorf(`¯\_(ツ)_/¯`)
			process := NewMockProcess(ctrl)
			process.EXPECT().
				Wait().
				// Wait as we return an error from post-start.
				Do(func() { time.Sleep(time.Second) }).
				Return(nil).
				AnyTimes()
			process.EXPECT().
				Signal(gomock.Any()).
				Return(nil).
				AnyTimes()
			runner := NewMockRunner(ctrl)
			gomock.InOrder(
				runner.EXPECT().
					Run(command, stdio).
					Return(process, nil).
					Times(1),
				runner.EXPECT().
					RunContext(gomock.Any(), gomock.Any(), stdio).
					Return(nil, expectedErr).
					Times(1),
			)
			checker := NewMockChecker(ctrl)
			checker.EXPECT().
				Check(postStart.Name).
				Return(true).
				Times(1)
			conditionRunner := NewMockRunner(ctrl)
			err := Run(runner, conditionRunner, checker, stdio, commandLine, postStart.Name, postStart.Arg, "", []string{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Errorf("failed to run container: %v", expectedErr).Error()))
		})

		It("fails when post-start Wait fails", func() {
			expectedErr := fmt.Errorf(`¯\_(ツ)_/¯`)
			process := NewMockProcess(ctrl)
			process.EXPECT().
				Wait().
				// Wait as we return an error from post-start.
				Do(func() { time.Sleep(time.Second) }).
				Return(nil).
				AnyTimes()
			process.EXPECT().
				Signal(gomock.Any()).
				Return(nil).
				AnyTimes()
			postStartProcess := NewMockProcess(ctrl)
			postStartProcess.EXPECT().
				Wait().
				Return(expectedErr).
				Times(1)
			postStartProcess.EXPECT().
				Signal(gomock.Any()).
				Return(nil).
				AnyTimes()
			runner := NewMockRunner(ctrl)
			gomock.InOrder(
				runner.EXPECT().
					Run(command, stdio).
					Return(process, nil).
					Times(1),
				runner.EXPECT().
					RunContext(gomock.Any(), gomock.Any(), stdio).
					Return(postStartProcess, nil).
					Times(1),
			)
			checker := NewMockChecker(ctrl)
			checker.EXPECT().
				Check(postStart.Name).
				Return(true).
				Times(1)
			conditionRunner := NewMockRunner(ctrl)
			err := Run(runner, conditionRunner, checker, stdio, commandLine, postStart.Name, postStart.Arg, "", []string{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Errorf("failed to run container: %v", expectedErr).Error()))
		})

		It("succeeds when main and post-start commands succeed", func() {
			var postStartWg sync.WaitGroup
			postStartWg.Add(1)
			process := NewMockProcess(ctrl)
			process.EXPECT().
				Wait().
				Do(postStartWg.Wait).
				Return(nil).
				AnyTimes()
			process.EXPECT().
				Signal(gomock.Any()).
				Return(nil).
				AnyTimes()
			postStartProcess := NewMockProcess(ctrl)
			postStartProcess.EXPECT().
				Wait().
				Do(postStartWg.Done).
				Return(nil).
				Times(1)
			postStartProcess.EXPECT().
				Signal(gomock.Any()).
				Return(nil).
				AnyTimes()
			runner := NewMockRunner(ctrl)
			gomock.InOrder(
				runner.EXPECT().
					Run(command, stdio).
					Return(process, nil).
					Times(1),
				runner.EXPECT().
					RunContext(gomock.Any(), gomock.Any(), stdio).
					Do(func(ctx context.Context, _ Command, _ Stdio) {
						_, ok := ctx.Deadline()
						Expect(ok).To(Equal(true))
					}).
					Return(postStartProcess, nil).
					Times(1),
			)
			checker := NewMockChecker(ctrl)
			checker.EXPECT().
				Check(postStart.Name).
				Return(true).
				Times(1)
			conditionRunner := NewMockRunner(ctrl)
			err := Run(runner, conditionRunner, checker, stdio, commandLine, postStart.Name, postStart.Arg, "", []string{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("With post-start condition", func() {
		It("fails when the condition fails", func() {
			expectedErr := fmt.Errorf(`¯\_(ツ)_/¯`)
			process := NewMockProcess(ctrl)
			process.EXPECT().
				Wait().
				// Wait as we return an error from post-start.
				Do(func() { time.Sleep(time.Second) }).
				Return(nil).
				AnyTimes()
			process.EXPECT().
				Signal(gomock.Any()).
				Return(nil).
				AnyTimes()
			runner := NewMockRunner(ctrl)
			runner.EXPECT().
				Run(command, stdio).
				Return(process, nil).
				Times(1)
			checker := NewMockChecker(ctrl)
			checker.EXPECT().
				Check(postStart.Name).
				Return(true).
				Times(1)
			conditionRunner := NewMockRunner(ctrl)
			conditionRunner.EXPECT().
				RunContext(gomock.Any(), postStartCondition, gomock.Any()).
				Return(nil, expectedErr).
				Times(1)
			err := Run(runner, conditionRunner, checker, stdio, commandLine, postStart.Name, postStart.Arg, postStartCondition.Name, postStartCondition.Arg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Errorf("failed to run container: %v", expectedErr).Error()))
		})

		It("succeeds when main and post-start commands succeed", func() {
			var postStartWg sync.WaitGroup
			postStartWg.Add(1)
			process := NewMockProcess(ctrl)
			process.EXPECT().
				Wait().
				Do(postStartWg.Wait).
				Return(nil).
				AnyTimes()
			process.EXPECT().
				Signal(gomock.Any()).
				Return(nil).
				AnyTimes()
			postStartProcess := NewMockProcess(ctrl)
			postStartProcess.EXPECT().
				Wait().
				Do(postStartWg.Done).
				Return(nil).
				Times(1)
			postStartProcess.EXPECT().
				Signal(gomock.Any()).
				Return(nil).
				AnyTimes()
			runner := NewMockRunner(ctrl)
			gomock.InOrder(
				runner.EXPECT().
					Run(command, stdio).
					Return(process, nil).
					Times(1),
				runner.EXPECT().
					RunContext(gomock.Any(), gomock.Any(), stdio).
					Do(func(ctx context.Context, _ Command, _ Stdio) {
						_, ok := ctx.Deadline()
						Expect(ok).To(Equal(true))
					}).
					Return(postStartProcess, nil).
					Times(1),
			)
			checker := NewMockChecker(ctrl)
			checker.EXPECT().
				Check(postStart.Name).
				Return(true).
				Times(1)
			conditionRunner := NewMockRunner(ctrl)
			conditionRunner.EXPECT().
				RunContext(gomock.Any(), postStartCondition, gomock.Any()).
				Do(func(ctx context.Context, _ Command, _ Stdio) {
					_, ok := ctx.Deadline()
					Expect(ok).To(Equal(true))
				}).
				Return(nil, nil).
				Times(1)
			err := Run(runner, conditionRunner, checker, stdio, commandLine, postStart.Name, postStart.Arg, postStartCondition.Name, postStartCondition.Arg)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("ProcessRegistry", func() {
	Context("NewProcessRegistry", func() {
		It("constructs a new ProcessRegistry", func() {
			pr := NewProcessRegistry()
			Expect(pr).ToNot(BeNil())
		})
	})

	Context("Register", func() {
		It("registers many processes", func() {
			pr := NewProcessRegistry()
			Expect(pr.Register(&ContainerProcess{})).To(Equal(1))
			Expect(pr.Register(&ContainerProcess{})).To(Equal(2))
			Expect(pr.Register(&ContainerProcess{})).To(Equal(3))
		})
	})

	Context("SignalAll", func() {
		var ctrl *gomock.Controller

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("fails when sending a signal to a process fails", func() {
			expectedErr := fmt.Errorf("failed to signal")

			p1 := NewMockProcess(ctrl)
			p2 := NewMockProcess(ctrl)
			p3 := NewMockProcess(ctrl)
			p4 := NewMockProcess(ctrl)

			pr := NewProcessRegistry()
			pr.Register(p1)
			pr.Register(p2)
			pr.Register(p3)
			pr.Register(p4)

			sig := syscall.SIGTERM
			p1.EXPECT().Signal(sig).Return(nil)
			p2.EXPECT().Signal(sig).Return(expectedErr)
			p3.EXPECT().Signal(sig).Return(nil)
			p4.EXPECT().Signal(sig).Return(expectedErr)

			errors := pr.SignalAll(sig)
			Expect(errors).To(Equal([]error{expectedErr, expectedErr}))
		})

		It("succeeds when all process signaling succeeds", func() {
			p1 := NewMockProcess(ctrl)
			p2 := NewMockProcess(ctrl)

			pr := NewProcessRegistry()
			pr.Register(p1)
			pr.Register(p2)

			sig := syscall.SIGTERM
			p1.EXPECT().Signal(sig).Return(nil)
			p2.EXPECT().Signal(sig).Return(nil)

			errors := pr.SignalAll(sig)
			Expect(errors).To(Equal([]error{}))
		})
	})

	Context("HandleSignals", func() {
		var ctrl *gomock.Controller

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("receives an error on the errors channel when signaling a process fails", func() {
			expectedErr := fmt.Errorf("failed to signal")
			sigs := make(chan os.Signal, 1)
			errors := make(chan error)
			sig := syscall.SIGTERM

			p1 := NewMockProcess(ctrl)

			pr := NewProcessRegistry()
			pr.Register(p1)

			p1.EXPECT().Signal(sig).Return(expectedErr)

			go pr.HandleSignals(sigs, errors)
			sigs <- sig
			err := <-errors
			Expect(err).To(Equal(expectedErr))
		})

		It("receives no error when signaling a process succeeds", func() {
			sigs := make(chan os.Signal, 1)
			errors := make(chan error)
			sig := syscall.SIGTERM

			p1 := NewMockProcess(ctrl)

			pr := NewProcessRegistry()
			pr.Register(p1)

			p1.EXPECT().Signal(sig).Return(nil)

			go pr.HandleSignals(sigs, errors)
			sigs <- sig
			Consistently(errors).ShouldNot(Receive())
		})
	})
})

var _ = Describe("ContainerProcess", func() {
	Context("NewContainerProcess", func() {
		It("constructs a new ContainerProcess", func() {
			cp := NewContainerProcess(nil)
			Expect(cp).ToNot(BeNil())
		})
	})

	Context("Signal", func() {
		var ctrl *gomock.Controller

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("is no-op if the process is not running", func() {
			p := NewMockOSProcess(ctrl)
			cp := NewContainerProcess(p)

			p.EXPECT().
				Signal(syscall.Signal(0)).
				Return(fmt.Errorf("not running")).
				Times(1)

			err := cp.Signal(syscall.SIGTERM)
			Expect(err).ToNot(HaveOccurred())
		})

		It("fails if signaling the unlerlying process fails", func() {
			p := NewMockOSProcess(ctrl)
			cp := NewContainerProcess(p)
			sig := syscall.SIGTERM

			gomock.InOrder(
				p.EXPECT().
					Signal(syscall.Signal(0)).
					Return(nil).
					Times(1),
				p.EXPECT().
					Signal(sig).
					Return(fmt.Errorf(`¯\_(ツ)_/¯`)).
					Times(1),
			)

			err := cp.Signal(sig)
			Expect(err).To(Equal(fmt.Errorf(`failed to send signal to process: ¯\_(ツ)_/¯`)))
		})

		It("succeeds when signaling the underlying process succeeds", func() {
			p := NewMockOSProcess(ctrl)
			cp := NewContainerProcess(p)
			sig := syscall.SIGTERM

			gomock.InOrder(
				p.EXPECT().
					Signal(syscall.Signal(0)).
					Return(nil).
					Times(1),
				p.EXPECT().
					Signal(sig).
					Return(nil).
					Times(1),
			)

			err := cp.Signal(sig)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Wait", func() {
		var ctrl *gomock.Controller

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("fails if the underlying process fails to wait", func() {
			p := NewMockOSProcess(ctrl)
			p.EXPECT().
				Wait().
				Return(nil, fmt.Errorf(`¯\_(ツ)_/¯`)).
				Times(1)

			cp := NewContainerProcess(p)
			err := cp.Wait()
			Expect(err).To(Equal(fmt.Errorf(`failed to run process: ¯\_(ツ)_/¯`)))
		})

		It("succeeds when the underlying process succeeds waiting", func() {
			state := &os.ProcessState{}
			p := NewMockOSProcess(ctrl)
			p.EXPECT().
				Wait().
				Return(state, nil).
				Times(1)

			cp := NewContainerProcess(p)
			err := cp.Wait()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("CommandChecker", func() {
	Context("NewCommandChecker", func() {
		It("constructs a new CommandChecker", func() {
			cc := NewCommandChecker(nil, nil)
			Expect(cc).ToNot(BeNil())
		})
	})

	Context("Check", func() {
		It("returns false when the command does not exist as a file neither in the $PATH", func() {
			osStat := func(string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			}
			execLookPath := func(file string) (string, error) {
				return "", exec.ErrNotFound
			}
			cc := NewCommandChecker(osStat, execLookPath)
			exists := cc.Check("a_command")
			Expect(exists).To(Equal(false))
		})

		It("returns true when the command exists as a file", func() {
			osStat := func(string) (os.FileInfo, error) {
				return nil, nil
			}
			execLookPath := func(file string) (string, error) {
				return "", exec.ErrNotFound
			}
			cc := NewCommandChecker(osStat, execLookPath)
			exists := cc.Check("a_command")
			Expect(exists).To(Equal(true))
		})

		It("returns true when the command exists in the $PATH", func() {
			osStat := func(string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			}
			execLookPath := func(file string) (string, error) {
				return "", nil
			}
			cc := NewCommandChecker(osStat, execLookPath)
			exists := cc.Check("a_command")
			Expect(exists).To(Equal(true))
		})
	})
})

var _ = Describe("ContainerRunner", func() {
	Context("NewContainerRunner", func() {
		It("constructs a new ContainerRunner", func() {
			cr := NewContainerRunner()
			Expect(cr).ToNot(BeNil())
		})
	})

	Context("Run", func() {
		It("fails to start a command that does not exist", func() {
			expectedErr := fmt.Errorf(`failed to run command: exec: "something_that_does_not_exist": executable file not found in $PATH`)
			cr := NewContainerRunner()
			cmd := Command{
				Name: "something_that_does_not_exist",
				Arg:  []string{},
			}
			stdio := Stdio{
				Out: ioutil.Discard,
				Err: ioutil.Discard,
			}
			p, err := cr.Run(cmd, stdio)
			Expect(err).To(Equal(expectedErr))
			Expect(p).To(BeNil())
		})

		It("fails when the command exits 1", func() {
			cr := NewContainerRunner()
			cmd := Command{
				Name: "bash",
				Arg:  []string{"-c", "exit 1"},
			}
			stdio := Stdio{
				Out: ioutil.Discard,
				Err: ioutil.Discard,
			}
			p, err := cr.Run(cmd, stdio)
			Expect(err).ToNot(HaveOccurred())
			Expect(p).ToNot(BeNil())
			err = p.Wait()
			Expect(err.Error()).To(Equal("failed to run process: exit status 1"))
		})

		It("succeeds", func() {
			Skip("this test needs to be fixed, it's currently flaky")
			cr := NewContainerRunner()
			cmd := Command{
				Name: "bash",
				Arg:  []string{"-c", ">&1 echo foo; >&2 echo bar; sleep 0.01"},
			}
			stdoutReader, stdoutWriter := io.Pipe()
			stderrReader, stderrWriter := io.Pipe()
			stdio := Stdio{
				Out: stdoutWriter,
				Err: stderrWriter,
			}
			p, err := cr.Run(cmd, stdio)
			Expect(err).ToNot(HaveOccurred())
			Expect(p).ToNot(BeNil())
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				stdout, err := ioutil.ReadAll(stdoutReader)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(stdout)).To(Equal("foo\n"))
				wg.Done()
			}()
			go func() {
				stderr, err := ioutil.ReadAll(stderrReader)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(stderr)).To(Equal("bar\n"))
				wg.Done()
			}()
			err = p.Wait()
			Expect(err).ToNot(HaveOccurred())
			stdoutWriter.Close()
			stderrWriter.Close()
			wg.Wait()
		})
	})

	Context("RunContext", func() {
		It("fails to start a command that does not exist", func() {
			expectedErr := fmt.Errorf(`failed to run command: exec: "something_that_does_not_exist": executable file not found in $PATH`)
			cr := NewContainerRunner()
			ctx := context.Background()
			cmd := Command{
				Name: "something_that_does_not_exist",
				Arg:  []string{},
			}
			stdio := Stdio{
				Out: ioutil.Discard,
				Err: ioutil.Discard,
			}
			p, err := cr.RunContext(ctx, cmd, stdio)
			Expect(err).To(Equal(expectedErr))
			Expect(p).To(BeNil())
		})

		It("succeeds", func() {
			Skip("this test needs to be fixed, it's currently flaky")
			cr := NewContainerRunner()
			ctx := context.Background()
			cmd := Command{
				Name: "bash",
				Arg:  []string{"-c", ">&1 echo foo; >&2 echo bar; sleep 0.01"},
			}
			stdoutReader, stdoutWriter := io.Pipe()
			stderrReader, stderrWriter := io.Pipe()
			stdio := Stdio{
				Out: stdoutWriter,
				Err: stderrWriter,
			}
			p, err := cr.RunContext(ctx, cmd, stdio)
			Expect(err).ToNot(HaveOccurred())
			Expect(p).ToNot(BeNil())
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				stdout, err := ioutil.ReadAll(stdoutReader)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(stdout)).To(Equal("foo\n"))
				wg.Done()
			}()
			go func() {
				stderr, err := ioutil.ReadAll(stderrReader)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(stderr)).To(Equal("bar\n"))
				wg.Done()
			}()
			err = p.Wait()
			Expect(err).ToNot(HaveOccurred())
			stdoutWriter.Close()
			stderrWriter.Close()
			wg.Wait()
		})
	})
})

var _ = Describe("ConditionRunner", func() {
	Context("NewConditionRunner", func() {
		It("constructs a new ConditionRunner", func() {
			cr := NewConditionRunner(nil, nil)
			Expect(cr).ToNot(BeNil())
		})
	})

	Context("Run", func() {
		It("is not implemented", func() {
			cr := NewConditionRunner(nil, nil)
			Expect(func() {
				cr.Run(Command{}, Stdio{})
			}).To(Panic())
		})
	})

	Context("RunContext", func() {
		var ctrl *gomock.Controller

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("fails when the context times out", func() {
			ctx := NewMockContext(ctrl)
			gomock.InOrder(
				ctx.EXPECT().
					Err().
					Return(nil).
					Times(1),
				ctx.EXPECT().
					Err().
					Return(context.DeadlineExceeded).
					Times(1),
			)
			cmd := Command{
				Name: "exit",
				Arg:  []string{"1"},
			}
			failCmd := exec.CommandContext(ctx, cmd.Name, cmd.Arg...)
			cc := NewMockExecCommandContext(ctrl)
			cc.EXPECT().
				CommandContext(ctx, cmd.Name, cmd.Arg[0]).
				Return(failCmd).
				Times(2)

			cr := NewConditionRunner(func(time.Duration) {}, cc.CommandContext)
			p, err := cr.RunContext(ctx, cmd, Stdio{})
			Expect(err).To(Equal(context.DeadlineExceeded))
			Expect(p).To(BeNil())
		})

		It("succeeds", func() {
			ctx := context.Background()
			cmd := Command{Name: "echo"}
			succeedCmd := exec.CommandContext(ctx, cmd.Name)
			cc := NewMockExecCommandContext(ctrl)
			cc.EXPECT().
				CommandContext(ctx, cmd.Name).
				Return(succeedCmd).
				Times(1)

			cr := NewConditionRunner(func(time.Duration) {}, cc.CommandContext)
			p, err := cr.RunContext(ctx, cmd, Stdio{})
			Expect(err).To(BeNil())
			Expect(p).To(BeNil())
		})
	})
})
