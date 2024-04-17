package main

import (
	"context"
	"os"

	cbleGRPC "github.com/cble-platform/cble-provider-grpc/pkg/cble"
	pgrpc "github.com/cble-platform/cble-provider-grpc/pkg/provider"
	"github.com/cble-platform/provider-openstack/openstack"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func main() {
	// TODO: Add CLI flags to allow non-default CBLE connect (e.g. TLS)

	// Check if the ID is passed in via command line
	if len(os.Args) < 2 {
		logrus.Errorf("no ID passed to provider")
		os.Exit(1)
	} else if len(os.Args) >= 3 {
		// Check for debug value
		if os.Args[2] == "DEBUG" {
			logrus.SetLevel(logrus.DebugLevel)
		}
	}
	id := os.Args[1]
	// Check the arg is a valid UUID (assume this is coming from ENT)
	if _, err := uuid.Parse(id); err != nil {
		logrus.Errorf("ID is not a valid UUID")
		os.Exit(2)
	}

	// Connect to the CBLE Provider gRPC Server
	conn, err := cbleGRPC.DefaultConnect()
	if err != nil {
		logrus.Fatalf("failed to connect to CBLE gRPC server: %v", err)
	}
	defer conn.Close()

	// Create the Openstack provider
	provider := openstack.ProviderOpenstack{}

	ctx := context.Background()

	// Create a CBLE Provider gRPC Server client
	client, err := cbleGRPC.NewClient(ctx, conn)
	if err != nil {
		logrus.Fatalf("failed to connect client: %v", err)
	}

	// Register this provider instance with the CBLE server
	registerReply, err := client.RegisterProvider(ctx, &cbleGRPC.RegistrationRequest{
		Id:      id,
		Name:    provider.Name(),
		Version: provider.Version(),
		Features: &cbleGRPC.ProviderFeatures{
			Deploy:  true,
			Destroy: true,
			Console: true,
		},
	})
	if err != nil || !registerReply.Success {
		logrus.Fatalf("registration failed: %v", err)
	} else {
		logrus.Printf("Registration success! Starting provider server on socket /tmp/cble-provider-grpc-%s", registerReply.SocketId)
	}

	providerOpts := &pgrpc.ProviderServerOptions{
		TLS:      false,
		CertFile: "",
		KeyFile:  "",
		SocketID: registerReply.SocketId,
	}

	logrus.Debugf("serving gRPC with socket ID %s", registerReply.SocketId)

	// Serve the provider gRPC server (blocking call until Ctrl+C)
	if err := pgrpc.Serve(provider, providerOpts); err != nil {
		logrus.Fatalf("failed to server provider gRPC server: %v", err)
	}

	// Time to shutdown
	unregisterReply, err := client.UnregisterProvider(ctx, &cbleGRPC.UnregistrationRequest{
		Id:      id,
		Name:    provider.Name(),
		Version: provider.Version(),
	})
	if err != nil || !unregisterReply.Success {
		logrus.Errorf("unregistration failed: %v", err)
	} else {
		logrus.Print("Unregistration success! Shutting down...")
	}
}
