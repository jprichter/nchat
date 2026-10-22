export default {
  baseBranch: 'master',
  branchPrefix: 'feat/',
  remote: 'origin',
  engines: {
    default: { name: 'codex', model: 'gpt-6.1-sol', effort: 'medium' },
    review: { name: 'codex', model: 'gpt-6-luna', effort: 'high' },
    docs: { name: 'claude', model: 'claude-haiku-5-5', effort: 'high' },
    'pr-description': { name: 'claude', model: 'claude-haiku-5-5', effort: 'high' },
  },
  phases: ['implement', 'docs', 'gate', 'review', 'address', 'pr-description', 'publish'],
  publish: { backend: 'github', draft: true },
  gate: [
    "rtk proxy python3 -m unittest discover -s src -p compose_test.py",
    "rtk proxy env GOMODCACHE=/tmp/chat-security-tools/mod GOCACHE=/tmp/chat-security-tools/build GOTOOLCHAIN=auto sh -c 'cd lib/wmchat/go && go test ./...'",
    "rtk proxy env GOMODCACHE=/tmp/chat-security-tools/mod GOCACHE=/tmp/chat-security-tools/build GOTOOLCHAIN=auto sh -c 'cd lib/wmchat/go && go vet ./...'",
    "rtk proxy env GOMODCACHE=/tmp/chat-security-tools/mod GOCACHE=/tmp/chat-security-tools/build GOTOOLCHAIN=auto sh -c 'cd lib/wmchat/go/ext/whatsmeow && go test ./...'",
  ],
  maxRounds: 5,
  timeoutMs: 60 * 60 * 1000,
  worktrees: true,
};
