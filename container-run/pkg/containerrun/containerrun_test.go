package containerrun_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/cf-operator/container-run/pkg/containerrun"
	. "code.cloudfoundry.org/cf-operator/container-run/pkg/containerrun/mocks"
)

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
			cp := NewContainerProcess(p)

			p.EXPECT().
				Wait().
				Return(nil, fmt.Errorf(`¯\_(ツ)_/¯`)).
				Times(1)

			err := cp.Wait()
			Expect(err).To(Equal(fmt.Errorf(`failed to wait for process: ¯\_(ツ)_/¯`)))
		})

		It("succeeds when the underlying process succeeds waiting", func() {
			p := NewMockOSProcess(ctrl)
			cp := NewContainerProcess(p)

			p.EXPECT().
				Wait().
				Return(nil, nil).
				Times(1)

			err := cp.Wait()
			Expect(err).ToNot(HaveOccurred())
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

		It("succeeds", func() {
			cr := NewContainerRunner()
			cmd := Command{
				Name: "bash",
				Arg:  []string{"-c", ">&1 echo foo; >&2 echo bar"},
			}
			var stdout, stderr strings.Builder
			stdio := Stdio{
				Out: &stdout,
				Err: &stderr,
			}
			p, err := cr.Run(cmd, stdio)
			Expect(err).ToNot(HaveOccurred())
			Expect(p).ToNot(BeNil())
			err = p.Wait()
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout.String()).To(Equal("foo\n"))
			Expect(stderr.String()).To(Equal("bar\n"))
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
			cr := NewContainerRunner()
			ctx := context.Background()
			cmd := Command{
				Name: "bash",
				Arg:  []string{"-c", ">&1 echo foo; >&2 echo bar"},
			}
			var stdout, stderr strings.Builder
			stdio := Stdio{
				Out: &stdout,
				Err: &stderr,
			}
			p, err := cr.RunContext(ctx, cmd, stdio)
			Expect(err).ToNot(HaveOccurred())
			Expect(p).ToNot(BeNil())
			err = p.Wait()
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout.String()).To(Equal("foo\n"))
			Expect(stderr.String()).To(Equal("bar\n"))
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
