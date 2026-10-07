# Go / Gon benchmark results

Run: 20261007T033706Z; macOS-27.0.1-arm64-arm-64bit-Mach-O; Apple M4.

10 runtime samples per variant, 300ms per benchmark/sample; 7 build samples. Positive percentages mean slower.

Every operation processes 64 elements. All benchmarks are shown, including regressions.

| Workload | Go legacy ns/op | Gon legacy ns/op | Gon modern ns/op | Modern / Gon legacy | B/op (Go / Gon / modern) | Allocs/op (Go / Gon / modern) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| CoalesceAssignMixedBatch | 55.3 | 47.2 | 46.3 | -1.7% [-1.9%, -1.4%] | 0 / 0 / 0 | 0 / 0 / 0 |
| ConditionalMixedBatch | 28.0 | 28.0 | 27.8 | -0.6% [-0.8%, -0.5%] | 0 / 0 / 0 | 0 / 0 / 0 |
| EnumConstructionBatch | 54.4 | 53.8 | 54.3 | +1.3% [-0.1%, +2.6%] | 0 / 0 / 0 | 0 / 0 / 0 |
| EnumCopyBatch | 50.4 | 50.9 | 50.6 | -0.4% [-1.5%, +0.0%] | 0 / 0 / 0 | 0 / 0 / 0 |
| ErrorHandlerMixedBatch | 1,069.5 | 1,074.0 | 1,079.0 | -0.0% [-1.2%, +1.8%] | 1024 / 1024 / 1024 | 32 / 32 / 32 |
| ErrorPropagationFailureBatch | 53.1 | 53.2 | 53.1 | -0.3% [-0.6%, -0.0%] | 0 / 0 / 0 | 0 / 0 / 0 |
| ErrorPropagationSuccessBatch | 73.5 | 76.0 | 70.7 | -4.2% [-11.3%, +1.6%] | 0 / 0 / 0 | 0 / 0 / 0 |
| InterpolationBatch | 3,636.0 | 3,575.0 | 3,537.5 | -1.3% [-2.3%, +0.3%] | 512 / 512 / 512 | 64 / 64 / 64 |
| LambdaCapturedBatch | 22.6 | 22.7 | 22.0 | -3.0% [-3.2%, -2.9%] | 0 / 0 / 0 | 0 / 0 / 0 |
| MatchingExhaustiveBatch | 126.0 | 125.7 | 114.7 | -8.8% [-10.6%, -7.9%] | 0 / 0 / 0 | 0 / 0 / 0 |
| MatchingGuardedBatch | 128.9 | 129.3 | 125.2 | -3.1% [-4.9%, -0.3%] | 0 / 0 / 0 | 0 / 0 / 0 |
| NamedReorderedBatch | 117.7 | 117.7 | 117.7 | +0.0% [-0.1%, +0.3%] | 0 / 0 / 0 | 0 / 0 / 0 |
| NamedVariadicBatch | 118.2 | 118.2 | 88.5 | -25.1% [-25.3%, -24.5%] | 0 / 0 / 0 | 0 / 0 / 0 |
| OptionAbsentBatch | 40.9 | 40.9 | 40.7 | -0.3% [-2.0%, +0.2%] | 0 / 0 / 0 | 0 / 0 / 0 |
| OptionAssignmentMixedBatch | 57.9 | 49.4 | 49.8 | +0.8% [+0.6%, +1.4%] | 0 / 0 / 0 | 0 / 0 / 0 |
| OptionPresentBatch | 37.5 | 37.4 | 37.5 | -0.0% [-0.2%, +0.1%] | 0 / 0 / 0 | 0 / 0 / 0 |
| PipelineFailureBatch | 5,103.5 | 5,130.0 | 5,111.5 | -0.6% [-1.2%, +1.7%] | 6400 / 6400 / 6400 | 160 / 160 / 160 |
| PipelineMixedBatch | 2,748.0 | 2,758.0 | 2,756.0 | -0.1% [-1.2%, +1.5%] | 3200 / 3200 / 3200 | 80 / 80 / 80 |
| PipelineSuccessBatch | 371.2 | 373.9 | 374.9 | +0.6% [-0.8%, +2.5%] | 0 / 0 / 0 | 0 / 0 / 0 |
| SafeCallMixedBatch | 86.7 | 96.5 | 97.0 | +0.6% [+0.2%, +1.5%] | 0 / 0 / 0 | 0 / 0 / 0 |
| SafeFieldAbsentBatch | 28.8 | 28.8 | 26.9 | -6.7% [-20.3%, +3.9%] | 0 / 0 / 0 | 0 / 0 / 0 |
| SafeFieldPresentBatch | 28.6 | 25.3 | 25.3 | -0.1% [-0.2%, +0.4%] | 0 / 0 / 0 | 0 / 0 / 0 |

## Representation

Sizes are bytes per value on this host. Retention is one untimed process sample per variant, with 10,000 present pointer payloads across a forced GC; heap deltas include runtime/allocator noise.

| Shape | Go legacy bytes | Gon legacy bytes | Gon modern bytes |
| --- | ---: | ---: | ---: |
| (*int)? | 16 | 16 | 16 |
| enum/record | 32 | 32 | 32 |
| enum/unit | 8 | 8 | 16 |
| int? | 16 | 16 | 16 |
| struct{}? | 2 | 2 | 16 |

| Variant | Slice storage bytes | Observed heap delta bytes | GC cycles | Checksum |
| --- | ---: | ---: | ---: | ---: |
| go-legacy | 160000 | 243824 | 1 | 49995000 |
| gon-legacy | 160000 | 243824 | 1 | 49995000 |
| gon-modern | 160000 | 243824 | 1 | 49995000 |

## Build

| Variant | Rebuild median seconds | No-change median seconds | Rebuild CPU seconds | Reported RSS MiB | Executable bytes |
| --- | ---: | ---: | ---: | ---: | ---: |
| go-legacy | 0.249 | 0.105 | 0.180 | 85.5 | 3819122 |
| gon-legacy | 0.249 | 0.109 | 0.180 | 82.7 | 3752418 |
| gon-modern | 0.251 | 0.115 | 0.180 | 82.7 | 3752466 |

## Reproduction and limits

- go: `go version go1.27.1 darwin/arm64`; compiler SHA-256 `da10adfefec48f7681707909e213b24c423f5f5aa7efebfcede7eabb1c3b4156`.
- gon: `go version go1.28-devel_d67f72b731 Mon Oct 5 16:12:50 2026 -0300 darwin/arm64`; compiler SHA-256 `98c08d17aea7d03fb88b21b4bdfacb79fb92fe0643f2af8d181ed8ab6308e124`.
- Fixture `common.go` SHA-256: `db8522782230b1d756be88d7c9bd2841915b034cc944f337edf5ea1144d8a3c1`.
- Fixture `common_test.go` SHA-256: `d59da31360ddcbce9536ebc80fe8798f5a590f7c194081409eb2e81bb522860c`.
- Fixture `legacy.go` SHA-256: `13a6af65c8d56ea546ce6d47fc78d398a4217edd05f52c319e94810307439f02`.
- Fixture `modern.go` SHA-256: `1eba07872b0a9ff3dcd4422c85649b46a26b364a85f36d28fe703e1853f43189`.

- Go clásico y Gon clásico usan fuentes idénticas (SHA-256 comprobado). Las tres variantes comparten datos, pruebas y benchmarks.
- Go y Gon tienen versiones base distintas: esa comparación mezcla cambios upstream, del runtime y de Gon. Gon moderno vs Gon clásico aísla la sintaxis en el mismo toolchain.
- Ejecución serial, orden de variantes aleatorio en cada ronda, GOMAXPROCS=1, CGO desactivado, optimizaciones por defecto; no se desactiva inlining.
- Cada ns/op, B/op y allocs/op corresponde a un lote de 64 elementos. Preparación de datos fuera del cronómetro; sumas consumidas por sinks globales. Contadores compartidos hacen observable la evaluación y agregan costo de instrumentación.
- Las asignaciones son bytes/objetos de heap por operación, no memoria total del proceso. Los buffers reutilizados no cuentan como asignaciones nuevas.
- Representation: tamaños de valor por unsafe.Sizeof y una muestra no temporizada de retención por variante, con 10.000 payloads puntero y un GC forzado. El delta de heap incluye ruido; no es un presupuesto ni una comparación estadística.
- Enums y opcionales nativos se comparan con structs tagged Go explícitos que distinguen presencia por tag, incluidos payloads nil presentes.
- Recompilación: dependencias calientes, cambio del entero buildNonce impreso por main que invalida el paquete principal y el enlace, compile+link con -p=1 y -trimpath. No mide construir el toolchain ni una compilación con toda la caché vacía.
- Build sin cambios usa la caché y el ejecutable existente. Wall time incluye el comando público y, en Gon, su launcher. CPU es user+system informado por /usr/bin/time.
- RSS es el máximo informado por /usr/bin/time, no la suma simultánea de memoria de todos los subprocesos. Ejecutables sin stripping; incluyen runtime y metadatos de depuración.
- Intervalos 95%: bootstrap descriptivo de medianas de ratios por ronda (5000 remuestreos). No corrigen múltiples comparaciones ni garantizan generalización.
- Resultados de una sola máquina/sesión, sin fijar núcleo ni controlar frecuencia, carga externa o estado térmico. No prueban rendimiento en otras arquitecturas o aplicaciones.

Correctness: 16 tests per variant; identical stdout:

```text
build=0 items_per_batch=64 checksum=46154 failures=176 effects={Reads:384 Validations:240 Defaults:212 Arguments:59 Locations:128 TrueBranches:80 FalseBranches:128 Constructions:64 Copies:64 MatchGuards:21 MatchBodies:128 OptionReads:128 OptionTransforms:64 OptionDefaults:96 Callees:64 NamedArguments:192 ArgumentTrace:6}
```

Raw evidence: [runtime samples](runtime-raw.json), [build samples](build-raw.json), [representation](representation.json), [summary](summary.json), [commands](commands.json), [HTML](report.html). Source copies and logs are in the same run directory.
