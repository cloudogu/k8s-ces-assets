package controller

import (
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type eventRecorder interface {
	record.EventRecorder
}

// used for mocks

//nolint:unused
//goland:noinspection GoUnusedType
type k8sClient interface {
	client.Client
}
