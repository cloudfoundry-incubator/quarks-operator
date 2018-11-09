package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	"code.cloudfoundry.org/cf-operator/pkg/operator"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func printVersion() {
	log.Printf("Go Version: %s", runtime.Version())
	log.Printf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
}

// WatchNamespaceEnvVar contains the namespace the operator should be watching for changes
const WatchNamespaceEnvVar = "WATCH_NAMESPACE"

func getWatchNamespace() (string, error) {
	ns, found := os.LookupEnv(WatchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", WatchNamespaceEnvVar)
	}
	return ns, nil
}

func main() {
	printVersion()
	flag.Parse()

	namespace, err := getWatchNamespace()
	if err != nil {
		log.Fatalf("failed to get watch namespace: %v", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	mgr, err := operator.Setup(cfg, manager.Options{Namespace: namespace})
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Starting the Cmd.")
	log.Fatal(mgr.Start(signals.SetupSignalHandler()))
}
