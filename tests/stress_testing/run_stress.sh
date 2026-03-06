#!/usr/bin/env bash
# Run pipeline stress tests with a 2-minute timeout.
# Use in CI or locally to validate success rate and SLO assertions.
# Exit code is that of go test (non-zero if any stress test fails).
set -e
cd "$(dirname "$0")/../.."
go test -timeout 2m -run 'TestHTTPStress_MockOfferEndpoint|TestMockPipeline_Stress|TestStressHarness_Realistic|TestMockPipeline_NoGoroutineLeak' ./tests/stress_testing/ "$@"
