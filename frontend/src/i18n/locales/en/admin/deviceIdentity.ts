export default {
  deviceIdentity: {
    title: 'Device Identity',
    description: 'Manage device identities for OpenAI and Claude accounts, including bulk initialization and rotation.',
    searchPlaceholder: 'Search account name or ID',
    selectAll: 'Select all',
    selected: '{count} selected',
    copy: 'Copy',
    copied: 'Copied',
    missing: 'Not configured',
    enabled: 'Enabled',
    disabled: 'Disabled',
    empty: 'No matching accounts',
    resetOne: 'Reset identity',
    generateMissing: 'Generate missing',
    resetAll: 'Reset all identities',
    enableTls: 'Enable TLS fingerprint',
    enableTlsHint: 'Applies to all Claude accounts that are not enabled yet',
    confirmEnableTls: 'Enable TLS fingerprint for {count} Claude accounts?',
    confirmGenerateMissing: 'Generate device identities for accounts that are missing one?',
    confirmResetAll: 'Reset device identities for all {count} accounts?',
    confirmResetOne: 'Reset the device identity for {name}?',
    resetSuccess: 'Device identity reset for {name}',
    ensureResult: 'Identity operation complete: updated {updated}, skipped {skipped}, failed {failed}',
    tlsResult: 'TLS fingerprint operation complete: {success} succeeded, {failed} failed',
    actionFailed: 'Operation failed. Please try again.',
    loadFailed: 'Failed to load accounts. Please try again.',
    progress: 'Running {done}/{total} · success {success} · failed {failed}',
    stats: {
      total: '{count} accounts',
      configured: '{count} configured',
      missing: '{count} missing'
    },
    filters: {
      allPlatforms: 'All platforms',
      allStatus: 'All status',
      missing: 'Missing',
      ready: 'Ready'
    },
    columns: {
      account: 'Account',
      platform: 'Platform',
      deviceId: 'Device identity ID',
      tls: 'TLS fingerprint',
      status: 'Status',
      actions: 'Actions'
    }
  }
}
