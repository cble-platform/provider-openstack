package main

import (
	"context"
	"os"

	cbleGRPC "github.com/cble-platform/cble-provider-grpc/pkg/cble"
	commonGRPC "github.com/cble-platform/cble-provider-grpc/pkg/common"
	providerGRPC "github.com/cble-platform/cble-provider-grpc/pkg/provider"
	"github.com/cble-platform/provider-openstack/openstack"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var (
	Name    = "provider-openstack"
	Version = "v1.0"
)

func main() {
	// TODO: Add CLI flags to allow non-default CBLE connect (e.g. TLS)

	// Check if the ID is passed in via command line
	if len(os.Args) < 2 {
		logrus.Errorf("no ID passed to provider")
		os.Exit(1)
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

	ctx := context.Background()

	// Create a CBLE Provider gRPC Server client
	client, err := cbleGRPC.NewClient(ctx, conn)
	if err != nil {
		logrus.Fatalf("failed to connect client: %v", err)
	}

	// Register this provider instance with the CBLE server
	registerReply, err := client.RegisterProvider(ctx, &cbleGRPC.RegistrationRequest{
		Id:      id,
		Name:    Name,
		Version: Version,
		Features: map[string]bool{
			providerGRPC.ProviderFeature_DEPLOY:  true,
			providerGRPC.ProviderFeature_DESTROY: true,
		},
	})
	if err != nil || registerReply.Status == commonGRPC.RPCStatus_FAILURE {
		logrus.Fatalf("registration failed: %v", err)
	} else if registerReply.Status == commonGRPC.RPCStatus_SUCCESS {
		logrus.Printf("Registration success! Starting provider server on socket /tmp/cble-provider-grpc-%s", registerReply.SocketId)
	} else {
		logrus.Fatalf("unknown error occurred: %v", err)
	}

	defer func() {
		// Time to shutdown
		unregisterReply, err := client.UnregisterProvider(ctx, &cbleGRPC.UnregistrationRequest{
			Id:      id,
			Name:    Name,
			Version: Version,
		})
		if err != nil || unregisterReply.Status == commonGRPC.RPCStatus_FAILURE {
			logrus.Fatalf("unregistration failed: %v", err)
		} else if unregisterReply.Status == commonGRPC.RPCStatus_SUCCESS {
			logrus.Print("Unregistration success! Shutting down...")
		} else {
			logrus.Fatalf("unknown error occurred: %v", err)
		}
	}()

	providerOpts := &providerGRPC.ProviderServerOptions{
		TLS:      false,
		CertFile: "",
		KeyFile:  "",
		SocketID: registerReply.SocketId,
	}

	// Serve the provider gRPC server
	if err := providerGRPC.Serve(openstack.ProviderOpenstack{}, providerOpts); err != nil {
		logrus.Fatalf("failed to server provider gRPC server: %v", err)
	}
}
