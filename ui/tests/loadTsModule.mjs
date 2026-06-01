import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import vm from 'node:vm'
import ts from 'typescript'

const rootDir = resolve(dirname(fileURLToPath(import.meta.url)), '..')

export function loadTsModule(relativePath) {
  const filename = resolve(rootDir, relativePath)
  const source = readFileSync(filename, 'utf8')
  const output = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
      esModuleInterop: true,
    },
    fileName: filename,
  }).outputText
  const module = { exports: {} }
  const context = vm.createContext({
    exports: module.exports,
    module,
    require: (id) => {
      throw new Error(`Unexpected runtime import ${id} from ${relativePath}`)
    },
    Set,
  })
  vm.runInContext(output, context, { filename })
  return module.exports
}
