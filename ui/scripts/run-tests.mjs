import { spawnSync } from 'node:child_process'

const batches = [
  [
    'tests/designModel.test.ts',
    'tests/designUtils.test.ts',
    'tests/formatAndStorage.test.ts',
    'tests/journeyModel.test.ts',
    'tests/navigation.test.ts',
    'tests/versionLifecycle.test.ts',
    'tests/backendApi.test.ts',
  ],
  [
    'tests/authScreens.test.tsx',
    'tests/formFields.test.tsx',
    'tests/statusAndErrors.test.tsx',
    'tests/homeScreen.test.tsx',
    'tests/appNavbar.test.tsx',
    'tests/canvasInteractions.test.ts',
    'tests/canvasLayout.test.ts',
    'tests/catalogGovernance.test.ts',
    'tests/designLifecycleModals.test.tsx',
  'tests/inspectorPanels.test.tsx',
  'tests/collaborationPanels.test.tsx',
  'tests/aiImportModals.test.tsx',
  'tests/richTextDocEditor.test.tsx',
  'tests/reactFlowCanvasProvider.test.tsx',
  ],
]

for (const files of batches) {
  const result = spawnSync(process.execPath, ['./node_modules/vitest/vitest.mjs', 'run', ...files], {
    cwd: process.cwd(),
    stdio: 'inherit',
  })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
}
