// Copyright 2025.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// SSH argument builders duplicated from adapters/ui/utils.go to avoid circular import.
// The sftp_client package (adapters/data) cannot import adapters/ui, so the shared
// SSH argument construction logic is duplicated here. Keep in sync with utils.go.
package sftp_client

import (
	"fmt"
	"strings"

	"github.com/Adembc/lazyssh/internal/core/domain"
)

// SSH config value constants (duplicated from adapters/ui/utils.go).
const (
	sshYes   = "yes"
	sshNo    = "no"
	sshForce = "force"
	sshAuto  = "auto"

	// SessionType values
	sessionTypeNone      = "none"
	sessionTypeSubsystem = "subsystem"
)

// buildSSHArgs constructs the SSH command arguments as a []string from a Server entity.
// This returns the arguments array (not a joined string), suitable for exec.Command.
// Logic mirrors BuildSSHCommand in adapters/ui/utils.go.
func buildSSHArgs(s domain.Server) []string {
	parts := []string{"ssh"}

	// Add proxy and connection options
	addProxyOptions(&parts, s)
	addConnectionTimingOptions(&parts, s)

	// Add port forwarding options
	addPortForwardingOptions(&parts, s)

	// Add authentication options
	addAuthOptions(&parts, s)

	// Add agent and forwarding options
	addForwardingOptions(&parts, s)

	// Add connection multiplexing options
	addMultiplexingOptions(&parts, s)

	// Add connection reliability options
	addConnectionOptions(&parts, s)

	// Add security options
	addSecurityOptions(&parts, s)

	// Add command execution options
	addCommandExecutionOptions(&parts, s)

	// Add environment options
	addEnvironmentOptions(&parts, s)

	// Add TTY and logging options
	addTTYAndLoggingOptions(&parts, s)

	// Port option
	if s.Port != 0 && s.Port != 22 {
		parts = append(parts, "-p", fmt.Sprintf("%d", s.Port))
	}

	// Identity file option
	if len(s.IdentityFiles) > 0 {
		for _, keyFile := range s.IdentityFiles {
			parts = append(parts, "-i", quoteIfNeeded(keyFile))
		}
	}

	// Host specification
	userHost := ""
	switch {
	case s.User != "" && s.Host != "":
		userHost = fmt.Sprintf("%s@%s", s.User, s.Host)
	case s.Host != "":
		userHost = s.Host
	default:
		userHost = s.Alias
	}
	parts = append(parts, userHost)

	// RemoteCommand (must come after the host)
	if s.RemoteCommand != "" {
		if s.RemoteCommand == sessionTypeNone {
			parts = append(parts, "-o", "RemoteCommand=none")
		} else {
			parts = append(parts, quoteIfNeeded(s.RemoteCommand))
		}
	}

	return parts
}

// addOption adds an SSH option in the format "-o Key=Value" if value is not empty.
func addOption(parts *[]string, key, value string) {
	if value != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("%s=%s", key, value))
	}
}

// addQuotedOption adds an SSH option with quoted value if needed.
func addQuotedOption(parts *[]string, key, value string) {
	if value != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("%s=%s", key, quoteIfNeeded(value)))
	}
}

// quoteIfNeeded returns the value quoted if it contains spaces.
func quoteIfNeeded(val string) string {
	if strings.ContainsAny(val, " \t") {
		return fmt.Sprintf("%q", val)
	}
	return val
}

// addProxyOptions adds proxy-related options to the SSH command.
func addProxyOptions(parts *[]string, s domain.Server) {
	if s.ProxyJump != "" {
		*parts = append(*parts, "-J", quoteIfNeeded(s.ProxyJump))
	}
	addQuotedOption(parts, "ProxyCommand", s.ProxyCommand)
}

// addConnectionTimingOptions adds connection timing options to the SSH command.
func addConnectionTimingOptions(parts *[]string, s domain.Server) {
	addOption(parts, "ConnectTimeout", s.ConnectTimeout)
	addOption(parts, "ConnectionAttempts", s.ConnectionAttempts)
	if s.BindAddress != "" {
		*parts = append(*parts, "-b", s.BindAddress)
	}
	if s.BindInterface != "" {
		*parts = append(*parts, "-B", s.BindInterface)
	}
	addOption(parts, "AddressFamily", s.AddressFamily)
	addOption(parts, "IPQoS", s.IPQoS)
	addOption(parts, "CanonicalizeHostname", s.CanonicalizeHostname)
	addOption(parts, "CanonicalDomains", s.CanonicalDomains)
	addOption(parts, "CanonicalizeFallbackLocal", s.CanonicalizeFallbackLocal)
	addOption(parts, "CanonicalizeMaxDots", s.CanonicalizeMaxDots)
	addQuotedOption(parts, "CanonicalizePermittedCNAMEs", s.CanonicalizePermittedCNAMEs)
}

// addPortForwardingOptions adds port forwarding options to the SSH command.
func addPortForwardingOptions(parts *[]string, s domain.Server) {
	for _, forward := range s.LocalForward {
		*parts = append(*parts, "-L", forward)
	}
	for _, forward := range s.RemoteForward {
		*parts = append(*parts, "-R", forward)
	}
	for _, forward := range s.DynamicForward {
		*parts = append(*parts, "-D", forward)
	}
	if s.ClearAllForwardings == sshYes {
		*parts = append(*parts, "-o", "ClearAllForwardings=yes")
	}
	if s.ExitOnForwardFailure == sshYes {
		*parts = append(*parts, "-o", "ExitOnForwardFailure=yes")
	}
	if s.GatewayPorts != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("GatewayPorts=%s", s.GatewayPorts))
	}
}

// addAuthOptions adds authentication-related options to the SSH command.
func addAuthOptions(parts *[]string, s domain.Server) {
	if s.PubkeyAuthentication != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("PubkeyAuthentication=%s", s.PubkeyAuthentication))
	}
	if s.PubkeyAcceptedAlgorithms != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("PubkeyAcceptedAlgorithms=%s", s.PubkeyAcceptedAlgorithms))
	}
	if s.HostbasedAcceptedAlgorithms != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("HostbasedAcceptedAlgorithms=%s", s.HostbasedAcceptedAlgorithms))
	}
	if s.PasswordAuthentication != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("PasswordAuthentication=%s", s.PasswordAuthentication))
	}
	if s.PreferredAuthentications != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("PreferredAuthentications=%s", s.PreferredAuthentications))
	}
	if s.IdentitiesOnly != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("IdentitiesOnly=%s", s.IdentitiesOnly))
	}
	if s.AddKeysToAgent != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("AddKeysToAgent=%s", s.AddKeysToAgent))
	}
	if s.IdentityAgent != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("IdentityAgent=%s", quoteIfNeeded(s.IdentityAgent)))
	}
	if s.KbdInteractiveAuthentication != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("KbdInteractiveAuthentication=%s", s.KbdInteractiveAuthentication))
	}
	if s.NumberOfPasswordPrompts != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("NumberOfPasswordPrompts=%s", s.NumberOfPasswordPrompts))
	}
}

// addForwardingOptions adds agent and X11 forwarding options to the SSH command.
func addForwardingOptions(parts *[]string, s domain.Server) {
	if s.ForwardAgent != "" {
		if s.ForwardAgent == sshYes {
			*parts = append(*parts, "-A")
		} else if s.ForwardAgent == sshNo {
			*parts = append(*parts, "-a")
		}
	}
	if s.ForwardX11 != "" {
		if s.ForwardX11 == sshYes {
			*parts = append(*parts, "-X")
		} else if s.ForwardX11 == sshNo {
			*parts = append(*parts, "-x")
		}
	}
	if s.ForwardX11Trusted == sshYes {
		*parts = append(*parts, "-Y")
	}
}

// addMultiplexingOptions adds connection multiplexing options to the SSH command.
func addMultiplexingOptions(parts *[]string, s domain.Server) {
	if s.ControlMaster != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("ControlMaster=%s", s.ControlMaster))
	}
	if s.ControlPath != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("ControlPath=%s", quoteIfNeeded(s.ControlPath)))
	}
	if s.ControlPersist != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("ControlPersist=%s", s.ControlPersist))
	}
}

// addConnectionOptions adds connection reliability options to the SSH command.
func addConnectionOptions(parts *[]string, s domain.Server) {
	if s.ServerAliveInterval != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("ServerAliveInterval=%s", s.ServerAliveInterval))
	}
	if s.ServerAliveCountMax != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("ServerAliveCountMax=%s", s.ServerAliveCountMax))
	}
	if s.Compression == sshYes {
		*parts = append(*parts, "-C")
	}
	if s.TCPKeepAlive != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("TCPKeepAlive=%s", s.TCPKeepAlive))
	}
	if s.BatchMode == sshYes {
		*parts = append(*parts, "-o", "BatchMode=yes")
	}
}

// addSecurityOptions adds security-related options to the SSH command.
func addSecurityOptions(parts *[]string, s domain.Server) {
	if s.StrictHostKeyChecking != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("StrictHostKeyChecking=%s", s.StrictHostKeyChecking))
	}
	if s.CheckHostIP != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("CheckHostIP=%s", s.CheckHostIP))
	}
	if s.FingerprintHash != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("FingerprintHash=%s", s.FingerprintHash))
	}
	if s.UserKnownHostsFile != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("UserKnownHostsFile=%s", quoteIfNeeded(s.UserKnownHostsFile)))
	}
	if s.HostKeyAlgorithms != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("HostKeyAlgorithms=%s", s.HostKeyAlgorithms))
	}
	if s.MACs != "" {
		*parts = append(*parts, "-m", s.MACs)
	}
	if s.Ciphers != "" {
		*parts = append(*parts, "-c", s.Ciphers)
	}
	if s.KexAlgorithms != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("KexAlgorithms=%s", s.KexAlgorithms))
	}
	if s.VerifyHostKeyDNS != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("VerifyHostKeyDNS=%s", s.VerifyHostKeyDNS))
	}
	if s.UpdateHostKeys != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("UpdateHostKeys=%s", s.UpdateHostKeys))
	}
	if s.HashKnownHosts != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("HashKnownHosts=%s", s.HashKnownHosts))
	}
	if s.VisualHostKey != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("VisualHostKey=%s", s.VisualHostKey))
	}
}

// addCommandExecutionOptions adds command execution options to the SSH command.
func addCommandExecutionOptions(parts *[]string, s domain.Server) {
	if s.LocalCommand != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("LocalCommand=%s", quoteIfNeeded(s.LocalCommand)))
	}
	if s.PermitLocalCommand != "" {
		*parts = append(*parts, "-o", fmt.Sprintf("PermitLocalCommand=%s", s.PermitLocalCommand))
	}
	if s.EscapeChar != "" {
		*parts = append(*parts, "-e", s.EscapeChar)
	}
}

// addEnvironmentOptions adds environment variable options to the SSH command.
func addEnvironmentOptions(parts *[]string, s domain.Server) {
	for _, env := range s.SendEnv {
		*parts = append(*parts, "-o", fmt.Sprintf("SendEnv=%s", env))
	}
	for _, env := range s.SetEnv {
		*parts = append(*parts, "-o", fmt.Sprintf("SetEnv=%s", quoteIfNeeded(env)))
	}
}

// addTTYAndLoggingOptions adds TTY and logging options to the SSH command.
func addTTYAndLoggingOptions(parts *[]string, s domain.Server) {
	if s.RequestTTY != "" {
		switch s.RequestTTY {
		case sshYes:
			*parts = append(*parts, "-t")
		case sshNo:
			*parts = append(*parts, "-T")
		case sshForce:
			*parts = append(*parts, "-tt")
		case sshAuto:
			// auto is the default behavior, no flag needed
		default:
			*parts = append(*parts, "-o", fmt.Sprintf("RequestTTY=%s", s.RequestTTY))
		}
	}

	if s.LogLevel != "" {
		switch strings.ToLower(s.LogLevel) {
		case "quiet":
			*parts = append(*parts, "-q")
		case "verbose":
			*parts = append(*parts, "-v")
		case "debug", "debug1":
			*parts = append(*parts, "-v")
		case "debug2":
			*parts = append(*parts, "-vv")
		case "debug3":
			*parts = append(*parts, "-vvv")
		}
	}

	if s.SessionType != "" {
		switch s.SessionType {
		case sessionTypeNone:
			*parts = append(*parts, "-N")
		case sessionTypeSubsystem:
			*parts = append(*parts, "-s")
		default:
			*parts = append(*parts, "-o", fmt.Sprintf("SessionType=%s", s.SessionType))
		}
	}
}
