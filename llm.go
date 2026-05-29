package eywa

import "github.com/wmulabs/eywa/internal/implementation/oracles"

// OracleFactoryImpl is the concrete Oracle factory. Register providers via
// RegisterProvider, then pass it to WeaveBuilder.WithOracleFactory.
type OracleFactoryImpl = oracles.Factory

// NewOracleFactory creates an OracleFactoryImpl. Register Oracle providers via
// OracleFactoryImpl.RegisterProvider, then pass it to WeaveBuilder.WithOracleFactory.
var NewOracleFactory = oracles.NewFactory
