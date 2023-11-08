package main

import (
	"context"

	cbleGRPC "github.com/cble-platform/cble-provider-grpc/pkg/cble"
	commonGRPC "github.com/cble-platform/cble-provider-grpc/pkg/common"
	providerGRPC "github.com/cble-platform/cble-provider-grpc/pkg/provider"
	"github.com/cble-platform/provider-openstack/openstack"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var (
	Id      = uuid.NewString() // Generate new ID on every boot up for freshness
	Name    = "provider-openstack"
	Version = "v1.0"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)

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
		Id:      Id,
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
		logrus.Printf("Registration success! Starting provider server on port %d", registerReply.Port)
	} else {
		logrus.Fatalf("unknown error occurred: %v", err)
	}

	defer func() {
		// Time to shutdown
		unregisterReply, err := client.UnregisterProvider(ctx, &cbleGRPC.UnregistrationRequest{
			Id:      Id,
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
		Port:     int(registerReply.Port),
	}

	// Serve the provider gRPC server
	if err := providerGRPC.Serve(openstack.ProviderOpenstack{}, providerOpts); err != nil {
		logrus.Fatalf("failed to server provider gRPC server: %v", err)
	}
}
