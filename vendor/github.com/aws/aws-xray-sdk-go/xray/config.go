// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package xray

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/aws/aws-xray-sdk-go/strategy/ctxmissing"
	"github.com/aws/aws-xray-sdk-go/strategy/exception"
	"github.com/aws/aws-xray-sdk-go/strategy/sampling"
	log "github.com/cihub/seelog"
)

// SDKVersion records the current X-Ray Go SDK version.
const SDKVersion = "1.0.0-rc.1"

// SDKType records which X-Ray SDK customer uses.
const SDKType = "X-Ray for Go"

// SDK provides the shape for unmarshalling an SDK struct.
type SDK struct {
	Version string `json:"sdk_version,omitempty"`
	Type    string `json:"sdk,omitempty"`
}

var globalCfg = newGlobalConfig()

func newGlobalConfig() *globalConfig {
	ret := &globalConfig{}

	// Set the logging configuration to the defaults
	ret.logLevel, ret.logFormat = loadLogConfig("", "")

	// Try to get the X-Ray daemon address from an environment variable
	if envDaemonAddr := os.Getenv("AWS_XRAY_DAEMON_ADDRESS"); envDaemonAddr != "" {

		// Try to resolve it, panic if it is malformed
		daemonAddress, err := net.ResolveUDPAddr("udp", envDaemonAddr)
		if err != nil {
			panic(err)
		}
		ret.daemonAddr = daemonAddress
	} else {
		// Use a default if the environment variable is empty
		ret.daemonAddr = &net.UDPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: 2000,
		}
	}

	ss, err := sampling.NewLocalizedStrategy()
	if err != nil {
		panic(err)
	}
	ret.samplingStrategy = ss

	efs, err := exception.NewDefaultFormattingStrategy()
	if err != nil {
		panic(err)
	}
	ret.exceptionFormattingStrategy = efs

	sts, err := NewDefaultStreamingStrategy()
	if err != nil {
		panic(err)
	}
	ret.streamingStrategy = sts

	cm := ctxmissing.NewDefaultRuntimeErrorStrategy()

	ret.contextMissingStrategy = cm

	return ret
}

type globalConfig struct {
	sync.RWMutex

	daemonAddr                  *net.UDPAddr
	serviceVersion              string
	samplingStrategy            sampling.Strategy
	streamingStrategy           StreamingStrategy
	exceptionFormattingStrategy exception.FormattingStrategy
	contextMissingStrategy      ctxmissing.Strategy
	logLevel                    log.LogLevel
	logFormat                   string
}

// Config is a set of X-Ray configurations.
type Config struct {
	DaemonAddr                  string
	ServiceVersion              string
	SamplingStrategy            sampling.Strategy
	StreamingStrategy           StreamingStrategy
	ExceptionFormattingStrategy exception.FormattingStrategy
	ContextMissingStrategy      ctxmissing.Strategy
	LogLevel                    string
	LogFormat                   string
}

// ContextWithConfig returns context with given configuration settings.
func ContextWithConfig(ctx context.Context, c Config) (context.Context, error) {
	var errors exception.MultiError

	var daemonAddress string
	if addr := os.Getenv("AWS_XRAY_DAEMON_ADDRESS"); addr != "" {
		daemonAddress = addr
	} else if c.DaemonAddr != "" {
		daemonAddress = c.DaemonAddr
	}

	if daemonAddress != "" {
		raddr, err := net.ResolveUDPAddr("udp", daemonAddress)
		if err == nil {
			go refreshEmitterWithAddress(raddr)
		} else {
			errors = append(errors, err)
		}
	}

	cms := os.Getenv("AWS_XRAY_CONTEXT_MISSING")
	if cms != "" {
		if cms == ctxmissing.RuntimeErrorStrategy {
			cm := ctxmissing.NewDefaultRuntimeErrorStrategy()
			c.ContextMissingStrategy = cm
		} else if cms == ctxmissing.LogErrorStrategy {
			cm := ctxmissing.NewDefaultLogErrorStrategy()
			c.ContextMissingStrategy = cm
		}
	}

	loadLogConfig(c.LogLevel, c.LogFormat)

	var err error
	switch len(errors) {
	case 0:
		err = nil
	case 1:
		err = errors[0]
	default:
		err = errors
	}

	return context.WithValue(ctx, RecorderContextKey{}, &c), err
}

// Configure overrides default configuration options with customer-defined values.
func Configure(c Config) error {
	globalCfg.Lock()
	defer globalCfg.Unlock()

	var errors exception.MultiError

	var daemonAddress string
	if addr := os.Getenv("AWS_XRAY_DAEMON_ADDRESS"); addr != "" {
		daemonAddress = addr
	} else if c.DaemonAddr != "" {
		daemonAddress = c.DaemonAddr
	}

	if daemonAddress != "" {
		addr, err := net.ResolveUDPAddr("udp", daemonAddress)
		if err == nil {
			globalCfg.daemonAddr = addr
			go refreshEmitter()
		} else {
			errors = append(errors, err)
		}
	}

	if c.SamplingStrategy != nil {
		globalCfg.samplingStrategy = c.SamplingStrategy
	}

	if c.ExceptionFormattingStrategy != nil {
		globalCfg.exceptionFormattingStrategy = c.ExceptionFormattingStrategy
	}

	if c.StreamingStrategy != nil {
		globalCfg.streamingStrategy = c.StreamingStrategy
	}

	cms := os.Getenv("AWS_XRAY_CONTEXT_MISSING")
	if cms != "" {
		if cms == ctxmissing.RuntimeErrorStrategy {
			cm := ctxmissing.NewDefaultRuntimeErrorStrategy()
			globalCfg.contextMissingStrategy = cm
		} else if cms == ctxmissing.LogErrorStrategy {
			cm := ctxmissing.NewDefaultLogErrorStrategy()
			globalCfg.contextMissingStrategy = cm
		}
	} else if c.ContextMissingStrategy != nil {
		globalCfg.contextMissingStrategy = c.ContextMissingStrategy
	}

	if c.ServiceVersion != "" {
		globalCfg.serviceVersion = c.ServiceVersion
	}

	globalCfg.logLevel, globalCfg.logFormat = loadLogConfig(c.LogLevel, c.LogFormat)

	switch len(errors) {
	case 0:
		return nil
	case 1:
		return errors[0]
	default:
		return errors
	}
}

func loadLogConfig(logLevel string, logFormat string) (log.LogLevel, string) {
	var level log.LogLevel
	var format string

	switch logLevel {
	case "trace":
		level = log.TraceLvl
	case "debug":
		level = log.DebugLvl
	case "info":
		level = log.InfoLvl
	case "warn":
		level = log.WarnLvl
	case "error":
		level = log.ErrorLvl
	default:
		level = log.InfoLvl
		logLevel = "info"
	}

	if logFormat != "" {
		format = logFormat
	} else {
		format = "%Date(2006-01-02T15:04:05Z07:00) [%Level] %Msg%n"
	}

	writer, _ := log.NewConsoleWriter()
	logger, err := log.LoggerFromWriterWithMinLevelAndFormat(writer, level, format)
	if err != nil {
		panic(fmt.Errorf("failed to create logs as StdOut: %v", err))
	}
	log.ReplaceLogger(logger)
	return level, format
}

func (c *globalConfig) DaemonAddr() *net.UDPAddr {
	c.RLock()
	defer c.RUnlock()
	return c.daemonAddr
}

func (c *globalConfig) SamplingStrategy() sampling.Strategy {
	c.RLock()
	defer c.RUnlock()
	return c.samplingStrategy
}

func (c *globalConfig) StreamingStrategy() StreamingStrategy {
	c.RLock()
	defer c.RUnlock()
	return c.streamingStrategy
}

func (c *globalConfig) ExceptionFormattingStrategy() exception.FormattingStrategy {
	c.RLock()
	defer c.RUnlock()
	return c.exceptionFormattingStrategy
}

func (c *globalConfig) ContextMissingStrategy() ctxmissing.Strategy {
	c.RLock()
	defer c.RUnlock()
	return c.contextMissingStrategy
}

func (c *globalConfig) ServiceVersion() string {
	c.RLock()
	defer c.RUnlock()
	return c.serviceVersion
}

func (c *globalConfig) LogLevel() log.LogLevel {
	c.RLock()
	defer c.RUnlock()
	return c.logLevel
}

func (c *globalConfig) LogFormat() string {
	c.RLock()
	defer c.RUnlock()
	return c.logFormat
}
