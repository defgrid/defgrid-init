package main

type ServiceStatus int

const (
	ServiceCritical ServiceStatus = iota
	ServiceWarning
	ServicePassing
)
