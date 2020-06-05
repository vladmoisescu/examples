package serviceregistry

type validationErrors []error

func (x *ServiceRequest) Validate() error {
	return nil
}

func (x *QueryRequest) Validate() error {
	return nil
}
