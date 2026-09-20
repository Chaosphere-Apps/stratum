export type CatalogAssetLike = {
  kind?: string
  status?: string
  updateMessage?: string
  replacementAssetId?: string
}

export function catalogAssetKindLabel(kind?: string) {
  return kind === 'system' ? 'System' : 'Component'
}

export function catalogAssetStatusLabel(status?: string) {
  switch ((status ?? 'active').trim().toLowerCase()) {
    case 'deprecated':
      return 'Deprecated'
    case 'retired':
      return 'Retired'
    case 'proposed':
      return 'Proposed'
    default:
      return 'Active'
  }
}

export function isCatalogAssetActionable(asset: CatalogAssetLike | null | undefined) {
  return asset?.status === 'deprecated' || asset?.status === 'retired'
}

export function catalogAssetAdvisoryText(asset: CatalogAssetLike | null | undefined) {
  if (!asset || !isCatalogAssetActionable(asset)) return ''
  if (asset.updateMessage?.trim()) return asset.updateMessage.trim()
  if (asset.replacementAssetId?.trim()) return `Use replacement asset ${asset.replacementAssetId.trim()}.`
  return asset.status === 'retired'
    ? 'This catalog asset is retired and should not be used in new designs.'
    : 'This catalog asset is deprecated. Review replacement guidance before using it.'
}
