export default {
  deviceIdentity: {
    title: '设备身份',
    description: '管理 OpenAI 与 Claude 账号的设备身份，并批量初始化或轮换身份。',
    searchPlaceholder: '搜索账号名称或 ID',
    selectAll: '全选',
    selected: '已选 {count} 个',
    copy: '复制',
    copied: '已复制',
    missing: '未配置',
    enabled: '已启用',
    disabled: '未启用',
    empty: '暂无符合条件的账号',
    resetOne: '重置身份',
    generateMissing: '生成缺失身份',
    resetAll: '全量重置身份',
    enableTls: '批量启用 TLS 指纹',
    enableTlsHint: '对所有尚未启用的 Claude 账号生效',
    confirmEnableTls: '确定为 {count} 个 Claude 账号启用 TLS 指纹吗？',
    confirmGenerateMissing: '确定为缺失身份的账号生成设备身份吗？',
    confirmResetAll: '确定重置全部 {count} 个账号的设备身份吗？',
    confirmResetOne: '确定重置账号 {name} 的设备身份吗？',
    resetSuccess: '账号 {name} 的设备身份已重置',
    ensureResult: '身份处理完成：更新 {updated}，跳过 {skipped}，失败 {failed}',
    tlsResult: 'TLS 指纹处理完成：成功 {success}，失败 {failed}',
    actionFailed: '操作失败，请稍后重试',
    loadFailed: '账号加载失败，请稍后重试',
    progress: '执行中 {done}/{total} · 成功 {success} · 失败 {failed}',
    stats: {
      total: '账号 {count}',
      configured: '已配置 {count}',
      missing: '待配置 {count}'
    },
    filters: {
      allPlatforms: '全部平台',
      allStatus: '全部状态',
      missing: '缺失',
      ready: '已就绪'
    },
    columns: {
      account: '账号',
      platform: '平台',
      deviceId: '设备身份 ID',
      tls: 'TLS 指纹',
      status: '状态',
      actions: '操作'
    }
  }
}
