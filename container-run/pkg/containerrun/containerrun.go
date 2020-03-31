package containerrun

//go:generate mockgen -destination=./mocks/mock_containerrun.go -package=mocks code.cloudfoundry.org/cf-operator/container-run/pkg/containerrun Runner,Checker,Process,OSProcess,ExecCommandContext,PacketListener,PacketConnection
//go:generate mockgen -destination=./mocks/mock_context.go -package=mocks context Context

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	postStartTimeout   = time.Minute * 15
	conditionSleepTime = time.Second * 3

	// ProcessStart is the command to restart the suspended child processes.
	ProcessStart = "START"
	// ProcessStop is the command to stop and suspend the child processes.
	ProcessStop = "STOP"
)

type processCommand string

// CmdRun represents the signature for the top-level Run command.
type CmdRun func(
	runner Runner,
	conditionRunner Runner,
	commandChecker Checker,
	listener PacketListener,
	stdio Stdio,
	args []string,
	jobName string,
	processName string,
	postStartCommandName string,
	postStartCommandArgs []string,
	postStartConditionCommandName string,
	postStartConditionCommandArgs []string,
) error

// Run implements the logic for the container-run CLI command.
func Run(
	runner Runner,
	conditionRunner Runner,
	commandChecker Checker,
	listener PacketListener,
	stdio Stdio,
	args []string,
	jobName string,
	processName string,
	postStartCommandName string,
	postStartCommandArgs []string,
	postStartConditionCommandName string,
	postStartConditionCommandArgs []string,
) error {
	if len(args) == 0 {
		err := fmt.Errorf("a command is required")
		return &runErr{err}
	}

	done := make(chan struct{}, 1)
	errors := make(chan error)
	sigs := make(chan os.Signal, 1)
	commands := make(chan processCommand)

	signal.Notify(sigs)
	processRegistry := NewProcessRegistry()

	command := Command{
		Name: args[0],
		Arg:  args[1:],
	}
	conditionCommand := Command{
		Name: postStartConditionCommandName,
		Arg:  postStartConditionCommandArgs,
	}
	postStartCommand := Command{
		Name: postStartCommandName,
		Arg:  postStartCommandArgs,
	}

	err := startProcesses(
		runner,
		conditionRunner,
		commandChecker,
		stdio,
		command,
		postStartCommand,
		conditionCommand,
		processRegistry,
		errors,
		done)
	if err != nil {
		return err
	}

	go processRegistry.HandleSignals(sigs, errors)

	// This flag records the state of the system and its child
	// processes. It is set to true when the child processes are
	// running, and false otherwise.
	active := true

	if err = watchForCommands(listener, jobName, processName, errors, commands); err != nil {
		return err
	}

	for {
		select {
		case cmd := <-commands:
			// Note: Commands are ignored if the system is
			// already in the requested state. I.e
			// demanding things to stop when things are
			// already stopped does nothing. Similarly for
			// demanding a start when the children are
			// started/up/active.

			switch cmd {
			case ProcessStop:
				if active {
					// Order is important here.
					// The `stopProcesses` sends
					// signals to the children,
					// which unlocks their Wait,
					// which causes events to be
					// posted on the done channel.
					// To properly ignore these
					// events the flag has to be
					// set before sending any
					// signals.

					active = false
					stopProcesses(processRegistry, errors)
				}
			case ProcessStart:
				if !active {
					err := startProcesses(
						runner,
						conditionRunner,
						commandChecker,
						stdio,
						command,
						postStartCommand,
						conditionCommand,
						processRegistry,
						errors,
						done)
					if err != nil {
						return err
					}

					active = true
				}
			}
		case <-done:
			// Ignore a done process when we actively
			// stopped the children via ProcessStop.
			if active {
				return nil
			}
		case err := <-errors:
			// Ignore done signals when we actively
			// stopped the children via ProcessStop.
			// Wait returns with !state.Sucess, `signal: killed`
			if active {
				return err
			}
		}
	}
}

func watchForCommands(
	listener PacketListener,
	jobName, processName string,
	errors chan error,
	commands chan processCommand,
) error {
	sockAddr := fmt.Sprintf("/var/vcap/data/%s/%s_containerrun.sock", jobName, processName)

	go func() {
		for {
			if err := os.RemoveAll(sockAddr); err != nil {
				errors <- fmt.Errorf("failed to setup command socket: %v", err)
				return
			}

			// Accept new packet, dispatching them to our handler
			packet, err := listener.ListenPacket("unixgram", sockAddr)
			if err != nil {
				errors <- fmt.Errorf("failed to watch for commands: %v", err)
			}
			if packet != nil {
				handlePacket(packet, errors, commands)
			}
		}
	}()

	return nil
}

func handlePacket(
	conn PacketConnection,
	errors chan error,
	commands chan processCommand,
) {
	defer conn.Close()

	packet := make([]byte, 256)
	n, _, err := conn.ReadFrom(packet)
	// Return address ignored. We do not send anything out.
	if err != nil && err != io.EOF {
		errors <- fmt.Errorf("failed to read command: %v", err)
	}

	command := strings.TrimSpace(string(packet[:n]))
	switch command {
	case ProcessStart, ProcessStop:
		commands <- processCommand(command)
	default:
		// Bad commands are ignored. Else they could be used to DOS the runner.
	}
}

func stopProcesses(processRegistry *ProcessRegistry, errors chan<- error) {
	for _, err := range processRegistry.SignalAll(os.Kill) {
		errors <- err
	}
}

func startProcesses(
	runner Runner,
	conditionRunner Runner,
	commandChecker Checker,
	stdio Stdio,
	command Command,
	postStartCommand Command,
	conditionCommand Command,
	processRegistry *ProcessRegistry,
	errors chan error,
	done chan struct{},
) error {
	err := startMainProcess(
		runner,
		command,
		stdio,
		processRegistry,
		errors,
		done)
	if err != nil {
		return err
	}

	startPostStartProcesses(
		runner,
		conditionRunner,
		commandChecker,
		stdio,
		postStartCommand,
		conditionCommand,
		processRegistry,
		errors)

	return nil
}

func startMainProcess(
	runner Runner,
	command Command,
	stdio Stdio,
	processRegistry *ProcessRegistry,
	errors chan error,
	done chan struct{},
) error {
	process, err := runner.Run(command, stdio)
	if err != nil {
		return &runErr{err}
	}
	processRegistry.Register(process)

	go func() {
		if err := process.Wait(); err != nil {
			errors <- &runErr{err}
			return
		}
		done <- struct{}{}
	}()

	return nil
}

func startPostStartProcesses(
	runner Runner,
	conditionRunner Runner,
	commandChecker Checker,
	stdio Stdio,
	postStartCommand Command,
	conditionCommand Command,
	processRegistry *ProcessRegistry,
	errors chan error,
) {
	if postStartCommand.Name != "" {
		if commandChecker.Check(postStartCommand.Name) {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), postStartTimeout)
				defer cancel()

				if conditionCommand.Name != "" {
					conditionStdio := Stdio{
						Out: ioutil.Discard,
						Err: ioutil.Discard,
					}

					if _, err := conditionRunner.RunContext(ctx, conditionCommand, conditionStdio); err != nil {
						errors <- &runErr{err}
						return
					}
				}

				postStartProcess, err := runner.RunContext(ctx, postStartCommand, stdio)
				if err != nil {
					errors <- &runErr{err}
					return
				}
				processRegistry.Register(postStartProcess)
				if err := postStartProcess.Wait(); err != nil {
					errors <- &runErr{err}
					return
				}
			}()
		}
	}
}

type runErr struct {
	err error
}

func (e *runErr) Error() string {
	return fmt.Sprintf("failed to run container: %v", e.err)
}

// Command represents a command to be run.
type Command struct {
	Name string
	Arg  []string
}

// Runner is the interface that wraps the Run methods.
type Runner interface {
	Run(command Command, stdio Stdio) (Process, error)
	RunContext(ctx context.Context, command Command, stdio Stdio) (Process, error)
}

// ContainerRunner satisfies the Runner interface.
type ContainerRunner struct {
}

// NewContainerRunner constructs a new ContainerRunner.
func NewContainerRunner() *ContainerRunner {
	return &ContainerRunner{}
}

// Run runs a command async.
func (cr *ContainerRunner) Run(
	command Command,
	stdio Stdio,
) (Process, error) {
	cmd := exec.Command(command.Name, command.Arg...)
	return cr.run(cmd, stdio)
}

// RunContext runs a command async with a context.
func (cr *ContainerRunner) RunContext(
	ctx context.Context,
	command Command,
	stdio Stdio,
) (Process, error) {
	cmd := exec.CommandContext(ctx, command.Name, command.Arg...)
	return cr.run(cmd, stdio)
}

func (cr *ContainerRunner) run(
	cmd *exec.Cmd,
	stdio Stdio,
) (Process, error) {
	cmd.Stdout = stdio.Out
	cmd.Stderr = stdio.Err
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to run command: %v", err)
	}
	return NewContainerProcess(cmd.Process), nil
}

// ConditionRunner satisfies the Runner interface. It represents a runner for a post-start
// pre-condition.
type ConditionRunner struct {
	sleep              func(time.Duration)
	execCommandContext func(context.Context, string, ...string) *exec.Cmd
}

// NewConditionRunner constructs a new ConditionRunner.
func NewConditionRunner(
	sleep func(time.Duration),
	execCommandContext func(context.Context, string, ...string) *exec.Cmd,
) *ConditionRunner {
	return &ConditionRunner{
		sleep:              sleep,
		execCommandContext: execCommandContext,
	}
}

// Run is not implemented.
func (cr *ConditionRunner) Run(
	command Command,
	stdio Stdio,
) (Process, error) {
	panic("not implemented")
}

// RunContext runs a condition until it succeeds or the context times out. The process is never
// returned. A context timeout makes RunContext to return the error.
func (cr *ConditionRunner) RunContext(
	ctx context.Context,
	command Command,
	_ Stdio,
) (Process, error) {
	for {
		cr.sleep(conditionSleepTime)
		cmd := cr.execCommandContext(ctx, command.Name, command.Arg...)
		if err := cmd.Run(); err != nil {
			if err := ctx.Err(); err == context.DeadlineExceeded {
				return nil, err
			}
			continue
		}
		break
	}

	return nil, nil
}

// Process is the interface that wraps the Signal and Wait methods of a process.
type Process interface {
	Signal(os.Signal) error
	Wait() error
}

// OSProcess is the interface that wraps the methods for *os.Process.
type OSProcess interface {
	Signal(os.Signal) error
	Wait() (*os.ProcessState, error)
}

// ContainerProcess satisfies the Process interface.
type ContainerProcess struct {
	process OSProcess
}

// NewContainerProcess constructs a new ContainerProcess.
func NewContainerProcess(process OSProcess) *ContainerProcess {
	return &ContainerProcess{
		process: process,
	}
}

// Signal sends a signal to the process. If the process is not running anymore, it's no-op.
func (p *ContainerProcess) Signal(sig os.Signal) error {
	// A call to ContainerProcess.Signal is no-op if the process it handles is not running.
	if err := p.process.Signal(syscall.Signal(0)); err != nil {
		return nil
	}
	if err := p.process.Signal(sig); err != nil {
		return fmt.Errorf("failed to send signal to process: %v", err)
	}
	return nil
}

// Wait waits for the process.
func (p *ContainerProcess) Wait() error {
	state, err := p.process.Wait()
	if err != nil {
		return fmt.Errorf("failed to run process: %v", err)
	} else if !state.Success() {
		err := &exec.ExitError{ProcessState: state}
		return fmt.Errorf("failed to run process: %v", err)
	}
	return nil
}

// Stdio represents the STDOUT and STDERR to be used by a process.
type Stdio struct {
	Out io.Writer
	Err io.Writer
}

// ProcessRegistry handles all the processes.
type ProcessRegistry struct {
	processes []Process
	sync.Mutex
}

// NewProcessRegistry constructs a new ProcessRegistry.
func NewProcessRegistry() *ProcessRegistry {
	return &ProcessRegistry{
		processes: make([]Process, 0),
	}
}

// Register registers a process in the registry and returns how many processes are registered.
func (pr *ProcessRegistry) Register(p Process) int {
	pr.Lock()
	defer pr.Unlock()
	pr.processes = append(pr.processes, p)
	return len(pr.processes)
}

// SignalAll sends a signal to all registered processes.
func (pr *ProcessRegistry) SignalAll(sig os.Signal) []error {
	pr.Lock()
	defer pr.Unlock()
	errors := make([]error, 0)
	for _, p := range pr.processes {
		if err := p.Signal(sig); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// HandleSignals handles the signals channel and forwards them to the
// registered processes. After a signal is handled it keeps running to
// handle any future ones.
func (pr *ProcessRegistry) HandleSignals(sigs <-chan os.Signal, errors chan<- error) {
	for {
		sig := <-sigs
		for _, err := range pr.SignalAll(sig) {
			errors <- err
		}
	}
}

// Checker is the interface that wraps the basic Check method.
type Checker interface {
	Check(command string) bool
}

// CommandChecker satisfies the Checker interface.
type CommandChecker struct {
	osStat       func(string) (os.FileInfo, error)
	execLookPath func(file string) (string, error)
}

// NewCommandChecker constructs a new CommandChecker.
func NewCommandChecker(
	osStat func(string) (os.FileInfo, error),
	execLookPath func(file string) (string, error),
) *CommandChecker {
	return &CommandChecker{
		osStat:       osStat,
		execLookPath: execLookPath,
	}
}

// Check checks if command exists as a file or in $PATH.
func (cc *CommandChecker) Check(command string) bool {
	_, statErr := cc.osStat(command)
	_, lookPathErr := cc.execLookPath(command)
	return statErr == nil || lookPathErr == nil
}

// ExecCommandContext wraps exec.CommandContext.
type ExecCommandContext interface {
	CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd
}

// PacketConnection is the interface that wraps the PacketConn methods.
type PacketConnection interface {
	ReadFrom(p []byte) (n int, addr net.Addr, err error)
	Close() error
}

// ListenPacketFunc is a type alias to the net.ListenPacket function.
type ListenPacketFunc func(network, address string) (net.PacketConn, error)

// PacketListener is the interface that wraps the ListenPacket methods.
// net.PacketConn satisfies this interface.
type PacketListener interface {
	ListenPacket(network, address string) (PacketConnection, error)
}

// NetPacketListener satisfies the PacketListener interface.
type NetPacketListener struct {
	listen ListenPacketFunc
}

// NewNetPacketListener constructs a new NetPacketListener.
func NewNetPacketListener(listen ListenPacketFunc) *NetPacketListener {
	return &NetPacketListener{
		listen: listen,
	}
}

// ListenPacket implements listening for packets.
func (npl *NetPacketListener) ListenPacket(network, address string) (PacketConnection, error) {
	conn, err := npl.listen(network, address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen for packet: %v", err)
	}
	return conn, nil
}
