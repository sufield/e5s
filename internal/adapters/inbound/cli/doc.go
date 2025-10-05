// Package cli provides an inbound adapter that drives the example
// application via a command-line interface.
//
// Responsibilities:
//   - Presentation and orchestration of application use-cases for demos/tests.
//   - No dependency wiring or configuration loading: those are provided by the
//     application composition root.
//
// The CLI adapter demonstrates how the hexagonal architecture separates
// concerns: the adapter interacts with the `app.Application` to perform
// attestation, fetch identity documents, and exercise core domain use-cases
// while keeping I/O and presentation logic out of the domain.
package cli
