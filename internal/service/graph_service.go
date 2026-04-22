// Package service previously contained GraphService, a thin wrapper around
// GraphRepository used exclusively by QueryTool. QueryTool now accepts
// service.CypherService directly, which provides the same behaviour with
// proper interface typing. GraphService has been retired to avoid duplication.
package service
