// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"context"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	kubetesting "k8s.io/client-go/testing"
)

func TestController_ensureValidToken(t *testing.T) {
	// Create a fake client with a CA secret
	fakeClient := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "k0s-ca",
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string][]byte{
				"ca.crt": []byte("-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA\n-----END CERTIFICATE-----"),
			},
		},
		&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes",
				Namespace: metav1.NamespaceDefault,
			},
			Subsets: []corev1.EndpointSubset{
				{
					Addresses: []corev1.EndpointAddress{
						{IP: "192.168.1.100"},
					},
					Ports: []corev1.EndpointPort{
						{Name: "https", Port: 6443, Protocol: corev1.ProtocolTCP},
					},
				},
			},
		},
	)

	clientFactory := &fakeClientFactory{client: fakeClient}

	controller, err := NewController(clientFactory)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	ctx := context.Background()

	// First call should create a new token
	err = controller.ensureValidToken(ctx)
	if err != nil {
		t.Fatalf("Failed to ensure valid token: %v", err)
	}

	if controller.lastToken == "" {
		t.Error("Expected token to be set")
	}

	initialToken := controller.lastToken
	initialTime := controller.lastTokenTime

	// Second call immediately after should not create a new token
	err = controller.ensureValidToken(ctx)
	if err != nil {
		t.Fatalf("Failed to ensure valid token on second call: %v", err)
	}

	if controller.lastToken != initialToken {
		t.Error("Expected token to remain the same on second call")
	}

	if controller.lastTokenTime != initialTime {
		t.Error("Expected token time to remain the same on second call")
	}

	// Simulate token expiration by setting time back
	controller.lastTokenTime = time.Now().Add(-24 * time.Hour)

	// Third call should create a new token
	err = controller.ensureValidToken(ctx)
	if err != nil {
		t.Fatalf("Failed to ensure valid token after expiration: %v", err)
	}

	if controller.lastToken == initialToken {
		t.Error("Expected new token after expiration")
	}
}

// fakeClientFactory implements kubernetes.ClientFactoryInterface for testing
type fakeClientFactory struct {
	client *fake.Clientset
}

func (f *fakeClientFactory) GetClient() (kubernetes.Interface, error) {
	return f.client, nil
}

func (f *fakeClientFactory) GetConfig() (*rest.Config, error) {
	return &rest.Config{}, nil
}

func (f *fakeClientFactory) GetRestConfig() (*rest.Config, error) {
	return &rest.Config{}, nil
}
