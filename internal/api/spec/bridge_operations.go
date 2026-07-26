package spec

func bridgeOperations() []OperationSpec {
	operations := bridgeCatalogOperations()
	operations = append(operations, bridgeRecordOperations()...)
	operations = append(operations, bridgeLifecycleOperations()...)
	operations = append(operations, bridgeTargetOperations()...)
	operations = append(operations, bridgeSecretOperations()...)
	operations = append(operations, bridgeDeliveryOperations()...)
	operations = append(operations, bridgeManifestOperations()...)
	operations = append(operations, bridgeControlOperations()...)
	return operations
}
