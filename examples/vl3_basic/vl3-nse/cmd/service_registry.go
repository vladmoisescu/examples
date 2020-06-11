package main

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/tiswanso/examples/api/serviceregistry"
	"google.golang.org/grpc"
)

type ServiceRegistry interface {
	RegisterWorkload(clusterName, podName, name, seviceName, connDom string, ipAddr []string, ports []int32) (error)
}

type ServiceRegistryImpl struct {
	registryClient serviceregistry.RegistryClient
}

func (s *ServiceRegistryImpl) RegisterWorkload(clusterName, podName, name, seviceName, connDom string, ipAddr []string, ports []int32) (error) {

	workloadIdentifier := &serviceregistry.WorkloadIdentifier{
		Cluster:             clusterName,
		PodName:             podName, // on request
		Name:                name, // No idea
	}

	workload := &serviceregistry.Workload{
		Identifier:          workloadIdentifier,
		IPAddress:           ipAddr, // on request
	}

	workloads := make([]*serviceregistry.Workload, 1)

	workloads[0] = workload

	serviceRequest := &serviceregistry.ServiceWorkload{
		ServiceName:         seviceName, // NetworkService
		ConnectivityDomain:  connDom, // on vxc
		Workloads:           workloads,
		Ports:               ports, // ce plm??
	}

	_, err := s.registryClient.RegisterWorkload(context.Background(), serviceRequest)
	if err != nil {
		logrus.Errorf("service registration not successful: %v", err)
	}

	return err
}

func NewServiceRegistry(addr string) (ServiceRegistry, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return &ServiceRegistryImpl{}, fmt.Errorf("unable to connect to ipam server: %v", err)
	}

	registryClient := serviceregistry.NewRegistryClient(conn)
	serviceRegistry := ServiceRegistryImpl{registryClient: registryClient}

	return &serviceRegistry, nil
}