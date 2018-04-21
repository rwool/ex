package signal

import (
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rwool/ex/log"
)

//go:generate stringer -type Signal

// Signal is a signal that can be handled with an SSH session.
type Signal uint8

// Signals that are supported by SSH.
const (
	SIGABRT Signal = iota
	SIGALRM
	SIGFPE
	SIGHUP
	SIGILL
	SIGINT
	SIGKILL
	SIGPIPE
	SIGQUIT
	SIGSEGV
	SIGTERM
	SIGUSR1
	SIGUSR2
)

var (
	stopC       = make(chan struct{})
	sigC        = make(chan Signal, 10)
	handlersSet bool
	// handlerMap is the mapping of signals to their handlers.
	handlerMap = map[Signal]func(Signal){}
	// Only set up signal handling once.
	handlerMu sync.Mutex

	sigToOSSig = map[Signal]os.Signal{
		SIGABRT: syscall.SIGABRT,
		SIGALRM: syscall.SIGALRM,
		SIGFPE:  syscall.SIGFPE,
		SIGHUP:  syscall.SIGHUP,
		SIGILL:  syscall.SIGILL,
		SIGINT:  syscall.SIGINT,
		SIGKILL: syscall.SIGKILL,
		SIGPIPE: syscall.SIGPIPE,
		SIGQUIT: syscall.SIGQUIT,
		SIGSEGV: syscall.SIGSEGV,
		SIGTERM: syscall.SIGTERM,
		SIGUSR1: syscall.SIGUSR1,
		SIGUSR2: syscall.SIGUSR2,
	}

	osSigToSig = map[os.Signal]Signal{
		syscall.SIGABRT: SIGABRT,
		syscall.SIGALRM: SIGALRM,
		syscall.SIGFPE:  SIGFPE,
		syscall.SIGHUP:  SIGHUP,
		syscall.SIGILL:  SIGILL,
		syscall.SIGINT:  SIGINT,
		syscall.SIGKILL: SIGKILL,
		syscall.SIGPIPE: SIGPIPE,
		syscall.SIGQUIT: SIGQUIT,
		syscall.SIGSEGV: SIGSEGV,
		syscall.SIGTERM: SIGTERM,
		syscall.SIGUSR1: SIGUSR1,
		syscall.SIGUSR2: SIGUSR2,
	}
)

// ErrHandlersAlreadySet indicates that an attempt was made to set the signal
// handlers after they were previously set.
var ErrHandlersAlreadySet = errors.New("signal handlers already set")

// setNoOpHandlers sets signal handlers that do nothing.
func setNoOpHandlers() {
	for sig := range sigToOSSig {
		handlerMap[sig] = func(Signal) {}
	}
}

func init() {
	setNoOpHandlers()
}

// BeginSignalHandling begins handling signals for this process.
//
// All signals listed by this package are handled.
//
// The defaultHandler is used for all signals that aren't explicitly given a
// handler function. If nil is passed for this, then a default handler that does
// nothing will be used.
//
// The functions given should not block for too long or handling of new signals
// will be delayed.
func BeginSignalHandling(logger log.Logger, defaultHandler func(Signal), signalHandlingPairs ...interface{}) error {
	if logger == nil {
		panic("nil logger")
	}

	handlerMu.Lock()
	defer handlerMu.Unlock()

	defer func() {
		// Undo signal handling setup if a panic occurs.
		if r := recover(); r != nil {
			setNoOpHandlers()
			handlersSet = false
			panic(r)
		}
	}()

	if handlersSet {
		return ErrHandlersAlreadySet
	}

	logger.Debugf("attempting to set up signal handling")

	handlersSet = true

	if defaultHandler == nil {
		defaultHandler = func(Signal) {}
	}

	shp := signalHandlingPairs

	if len(shp)%2 == 1 {
		panic("odd number of handling pairs given")
	}

	var currentSignal Signal
	var isOdd bool
	signalsSet := map[Signal]struct{}{}
	for i := range shp {
		if isOdd {
			// Function.
			if v, ok := shp[i].(func(Signal)); ok {
				logger.Debugf("registering signal for %s", currentSignal.String())
				handlerMap[currentSignal] = v
			} else {
				panic("not a func() for odd argument")
			}
		} else {
			// Signal.
			if v, ok := shp[i].(Signal); ok {
				currentSignal = v
				signalsSet[v] = struct{}{}
			} else {
				if _, ok := shp[i].(syscall.Signal); ok {
					panic("gave syscall.Signal for argument but expected Signal from this package")
				}
				panic("not a Signal for even argument")
			}
		}
		isOdd = !isOdd
	}

	// Set the default handler for all signals that haven't gotten a signal
	// set.
	for k := range sigToOSSig {
		if _, ok := signalsSet[k]; !ok {
			handlerMap[k] = defaultHandler
		}
	}

	// WaitGroup to wait for signal handling to actually be ready.
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		osSigC := make(chan os.Signal, 10)

		signal.Notify(osSigC)
		logger.Debugf("all OS signals now being handled")
		defer signal.Stop(osSigC)
		wg.Done()
		for {
			select {
			case sig := <-osSigC:
				logger.Debugf("handling OS signal: %s", sig.String())
				var nonOSSig Signal
				var ok bool
				if nonOSSig, ok = osSigToSig[sig]; !ok {
					continue
				}
				func() {
					defer func() {
						if r := recover(); r != nil {
							logger.Errorf("panic from calling handler for OS signal %s: %+v", sig.String(), r)
						}
					}()
					handlerMap[osSigToSig[sig]](nonOSSig)
				}()
			case sig := <-sigC:
				logger.Debugf("handling non-OS signal: %s", sig.String())
				func() {
					defer func() {
						if r := recover(); r != nil {
							logger.Errorf("panic from calling handler for non-OS signal %s: %+v", sig.String(), r)
						}
					}()
					handlerMap[sig](sig)
				}()
			case <-stopC:
				logger.Debugf("stopping signal handling loop")
				return
			}
		}

	}()

	wg.Wait()
	return nil
}

// SendSignal sends the given signal without having to use a "real" OS signal.
//
// If signal handling hasn't been set up yet, then calls to this function will
// do nothing.
func SendSignal(s Signal) {
	sigC <- s
}

// StopSignalHandling will stop handling signals.
//
// Will block forever if signals are not being handled.
func StopSignalHandling() {
	handlerMu.Lock()
	defer handlerMu.Unlock()

	stopC <- struct{}{}

	setNoOpHandlers()
	handlersSet = false

	sigs := make([]os.Signal, len(sigToOSSig))
	var index int
	for sig := range sigToOSSig {
		sigs[index] = sigToOSSig[sig]
		index++
	}
	signal.Reset(sigs...)
}
