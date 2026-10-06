# cw-p — unresolved findings and candidates

Every finding below is still reported. A candidate is a liberal match offered for a person to weigh, never an answer the resolver took: **high** is the only component declaring every method the function calls on the receiver (or the only one, named like it); **medium** is a sole match on weaker evidence, or the one named like the receiver among several; **low** is one of several. *Defined* is how many indexed files declare a method of that name: 0 means it is missing from the workspace, more means the resolver could not connect the call to it.

| Category | Findings | high | medium | low | none | method defined nowhere |
|---|---:|---:|---:|---:|---:|---:|
| variable | 1800 | 392 | 305 | 603 | 500 | 306 |
| return-type | 148 | 18 | 43 | 74 | 13 | 11 |
| method | 210 | 0 | 100 | 75 | 35 | 32 |
| object | 218 | 0 | 1 | 2 | 215 | 3 |

## Variable definitions — a receiver whose component is unknown

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 66 | `variables.helpers → cli/lucli/services/Helpers.cfc` | 7 | low | declares pluralize(), capitalize(); named like the receiver 'helpers' (3 candidates) | `cli/lucli/services/Admin.cfc:31` variable 'variables.helpers' has no component ref |
| 55 | `application.wheels.engineadapter → vendor/wheels/engineAdapters/Base.cfc` | 2 | low | declares parseFormKey() (2 candidates) | `vendor/wheels/Dispatch.cfc:150` variable 'application.wheels.engineAdapter' has no component ref |
| 53 | `details → cli/src/models/DetailOutputService.cfc` | 1 | high | declares header(), skip() | `cli/src/commands/wheels/generate/app-wizard.cfc:109` variable 'details' has no component ref |
| 46 | `migration.adapter → vendor/wheels/databaseAdapters/CockroachDB/CockroachDBMigrator.cfc` | 7 | low | declares adapterName() (7 candidates) | `vendor/wheels/tests/specs/database/CockroachDBCrudSpec.cfc:11` variable 'migration.adapter' has no component ref |
| 41 | `ctx.g` | 1 | none |  | `vendor/wheels/tests/specs/global/GlobalSurfaceIdentitySpec.cfc:42` variable 'ctx.g' has no component ref |
| 40 | `local.rs` | 0 | none |  | `cli/src/commands/wheels/base.cfc:1091` variable 'local.rs' has no component ref |
| 40 | `local.stmt` | 0 | none |  | `cli/src/commands/wheels/base.cfc:1090` variable 'local.stmt' has no component ref |
| 36 | `rl → tools/article-tests/Probe.cfc` | 16 | low | declares handle() (16 candidates) | `tools/article-tests/edge-cases.cfm:44` variable 'rl' has no component ref |
| 36 | `variables.config → cli/lucli/services/deploy/config/Config.cfc` | 1 | high | declares destination(); named like the receiver 'config' | `cli/lucli/services/deploy/commands/AccessoryCommands.cfc:72` variable 'variables.config' has no component ref |
| 34 | `arguments.printer` | 2 | none |  | `cli/src/models/AnalysisService.cfc:186` variable 'arguments.printer' has no component ref |
| 30 | `ssh → cli/lucli/services/deploy/lib/SshClient.cfc` | 1 | high | declares uploadString(), run() | `cli/lucli/services/deploy/cli/DeployAccessoryCli.cfc:188` variable 'ssh' has no component ref |
| 29 | `state.publiccfc → vendor/wheels/Public.cfc` | 1 | high | declares $cliCommandIsMutating(), $cliMutationGateCheck(), $cliResolveDumpPath(), $cliFormatMigrationStatus(), $cliDatabaseType() | `vendor/wheels/tests/specs/security/CliEndpointHardeningSpec.cfc:70` variable 'state.publicCfc' has no component ref |
| 26 | `variables.wheels.class.adapter → vendor/wheels/databaseAdapters/Base.cfc` | 2 | low | declares $setSharedModel(), $getColumns() (2 candidates) | `vendor/wheels/Model.cfc:202` variable 'variables.wheels.class.adapter' has no component ref |
| 24 | `adapter → vendor/wheels/engineAdapters/Base.cfc` | 2 | low | declares invokeMethod() (2 candidates) | `vendor/wheels/tests/specs/dispatch/InvokeMethodSpec.cfc:22` variable 'adapter' has no component ref |
| 20 | `mapper → vendor/wheels/mapper/mapping.cfc` | 1 | medium | declares $draw() | `vendor/wheels/tests/specs/global/urlforSpec.cfc:29` variable 'mapper' has no component ref |
| 20 | `variables.sshpool → cli/lucli/services/deploy/lib/SshPool.cfc` | 2 | medium | declares onEach(); named like the receiver 'sshPool' (2 candidates) | `cli/lucli/services/deploy/cli/DeployAccessoryCli.cfc:119` variable 'variables.sshPool' has no component ref |
| 19 | `arguments.injector → vendor/wheels/Injector.cfc` | 4 | low | declares to() (4 candidates) | `vendor/wheels/Bindings.cfc:9` variable 'arguments.injector' has no component ref |
| 18 | `arguments.conn` | 0 | none |  | `cli/src/commands/wheels/base.cfc:1088` variable 'arguments.conn' has no component ref |
| 18 | `local.conn` | 0 | none |  | `cli/src/commands/wheels/base.cfc:1205` variable 'local.conn' has no component ref |
| 18 | `role → cli/lucli/services/deploy/config/Role.cfc` | 9 | medium | declares name(), hosts(); named like the receiver 'role' (2 candidates) | `cli/lucli/services/deploy/cli/DeployAppCli.cfc:143` variable 'role' has no component ref |
| 17 | `arguments.accessory → cli/lucli/services/deploy/config/Accessory.cfc` | 1 | high | declares containerName(), image(), cmd(); named like the receiver 'accessory' | `cli/lucli/services/deploy/commands/AccessoryCommands.cfc:21` variable 'arguments.accessory' has no component ref |
| 17 | `ssh → cli/lucli/services/TestRunner.cfc` | 706 | low | declares run() (702 candidates) | `cli/lucli/services/deploy/cli/DeployAccessoryCli.cfc:119` variable 'ssh' has no component ref |
| 16 | `binder → vendor/wheels/Injector.cfc` | 4 | low | declares to() (4 candidates) | `cli/src/ModuleConfig.cfc:44` variable 'binder' has no component ref |
| 16 | `object` | 1 | none |  | `vendor/wheels/tests/specs/internal/model/validationsSpec.cfc:21` variable 'object' has no component ref |
| 15 | `application.wheelsdi → vendor/wheels/Injector.cfc` | 3 | low | declares getInstance() (2 candidates) | `vendor/wheels/Test.cfc:546` variable 'application.wheelsdi' has no component ref |
| 15 | `variables.modelreference → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/model/query/QueryBuilder.cfc:341` variable 'variables.modelReference' has no component ref |
| 14 | `arguments.associatedclass → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/model/sql.cfm:1653` variable 'arguments.associatedClass' has no component ref |
| 14 | `arguments.context.migrator → vendor/wheels/Migrator.cfc` | 3 | medium | declares createMigration(); named like the receiver 'migrator' (3 candidates) | `vendor/wheels/public/CliBridge.cfc:107` variable 'arguments.context.migrator' has no component ref |
| 14 | `user.birthtime` | 0 | none |  | `vendor/wheels/tests/specs/model/propertiesSpec.cfc:110` variable 'user.birthTime' has no component ref |
| 13 | `arguments.printer → cli/src/models/DetailOutputService.cfc` | 1 | medium | declares line() | `cli/src/models/AnalysisService.cfc:58` variable 'arguments.printer' has no component ref |
| 12 | `application[].pluginobj → vendor/wheels/Plugins.cfc` | 1 | high | declares getPlugins(), getPluginMeta(), getIncompatiblePlugins(), getDependantPlugins(), getVersionMismatchPlugins(), getMixinCollisions(), getMixins(), getP… | `vendor/wheels/global/plugins.cfm:335` variable 'application[].PluginObj' has no component ref |
| 12 | `stack.cache → cli/lucli/services/packages/ManifestCache.cfc` | 7 | low | declares refresh() (7 candidates) | `cli/lucli/tests/specs/packages/PackagesMainCliSpec.cfc:76` variable 'stack.cache' has no component ref |
| 12 | `variables.modelreference → vendor/wheels/model/query/QueryBuilder.cfc` | 4 | low | declares findOne() (3 candidates) | `vendor/wheels/model/query/QueryBuilder.cfc:363` variable 'variables.modelReference' has no component ref |
| 12 | `variables.page` | 0 | none |  | `vendor/wheels/wheelstest/BrowserClient.cfc:39` variable 'variables.page' has no component ref |
| 11 | `acc → cli/lucli/services/deploy/config/Accessory.cfc` | 4 | low | declares env(), hosts(), name() (2 candidates) | `cli/lucli/services/deploy/cli/DeployAccessoryCli.cfc:77` variable 'acc' has no component ref |
| 11 | `application[].packageloaderobj → vendor/wheels/PackageLoader.cfc` | 1 | high | declares getPackages(), getPackageMeta(), getFailedPackages(), getMixinCollisions(), getMixins(), getMethodProviders(), getPackageMiddleware(), $hasServicePr… | `vendor/wheels/global/plugins.cfm:379` variable 'application[].PackageLoaderObj' has no component ref |
| 11 | `details` | 0 | none |  | `cli/src/commands/wheels/generate/app.cfc:91` variable 'details' has no component ref |
| 11 | `map → vendor/wheels/mapper/resources.cfc` | 2 | low | declares resources(), resource() (2 candidates) | `vendor/wheels/tests/specs/mapper/NestedResourcesSpec.cfc:24` variable 'map' has no component ref |
| 10 | `arguments.callbacks` | 0 | none |  | `vendor/wheels/wheelstest/system/TestBox.cfc:757` variable 'arguments.callbacks' has no component ref |
| 10 | `codegen → cli/lucli/services/CodeGen.cfc` | 2 | medium | declares validateName(), generateModel(); named like the receiver 'codegen' (2 candidates) | `cli/lucli/Module.cfc:4797` variable 'codegen' has no component ref |
| 10 | `modulerecord.moduleconfig` | 1 | none |  | `vendor/wheels/wheelstest/system/TestBox.cfc:194` variable 'moduleRecord.moduleConfig' has no component ref |
| 9 | `application.wheels.migrator → vendor/wheels/Migrator.cfc` | 2 | medium | declares migrateToLatest(); named like the receiver 'migrator' (2 candidates) | `cli/lucli/templates/app/tests/populate.cfm:38` variable 'application.wheels.migrator' has no component ref |
| 9 | `arguments.column → vendor/wheels/migrator/ColumnDefinition.cfc` | 2 | low | declares toSQL() (2 candidates) | `vendor/wheels/databaseAdapters/Abstract.cfc:230` variable 'arguments.column' has no component ref |
| 9 | `l → vendor/wheels/wheelstest/BrowserLauncher.cfc` | 2 | high | declares getState(), $classpathJarPaths(), resolveInstallDir(), $loadJars() | `vendor/wheels/tests/specs/wheelstest/BrowserLauncherSpec.cfc:29` variable 'l' has no component ref |
| 9 | `local.response` | 0 | none |  | `vendor/wheels/controller/sse.cfc:72` variable 'local.response' has no component ref |
| 9 | `response` | 0 | none |  | `vendor/wheels/controller/rendering.cfc:171` variable 'response' has no component ref |
| 9 | `variables.$sshj` | 0 | none |  | `cli/lucli/services/deploy/lib/SshClient.cfc:70` variable 'variables.$sshj' has no component ref |
| 9 | `variables.codegenservice → cli/lucli/services/CodeGen.cfc` | 3 | low | declares generateModel(), generateController(), generateView(), generateTest() (3 candidates) | `cli/lucli/services/Scaffold.cfc:74` variable 'variables.codeGenService' has no component ref |
| 8 | `arguments.value` | 0 | none |  | `vendor/wheels/global/util.cfm:320` variable 'arguments.value' has no component ref |
| 8 | `local.controller` | 1 | none |  | `vendor/wheels/global/request.cfm:535` variable 'local.controller' has no component ref |
| 8 | `manager → vendor/wheels/storage/StorageManager.cfc` | 1 | high | declares disk(), getDefaultDiskName(), diskNames() | `vendor/wheels/tests/specs/storage/StorageSpec.cfc:483` variable 'manager' has no component ref |
| 8 | `mapper → vendor/wheels/mapper/matching.cfc` | 1 | high | declares $match(), wildcard() | `vendor/wheels/tests/specs/global/urlforSpec.cfc:29` variable 'mapper' has no component ref |
| 8 | `s3 → vendor/wheels/interfaces/StorageDiskInterface.cfc` | 3 | low | declares url(), signedUrl() (3 candidates) | `vendor/wheels/tests/specs/storage/StorageSpec.cfc:323` variable 's3' has no component ref |
| 8 | `stack.cli → cli/lucli/services/packages/PackagesMainCli.cfc` | 3 | high | declares list(), search(), show(), install(), remove() | `cli/lucli/tests/specs/packages/PackagesMainCliSpec.cfc:73` variable 'stack.cli' has no component ref |
| 8 | `variables.installer → cli/lucli/services/packages/Installer.cfc` | 1 | high | declares isInstalled(), installedVersion(); named like the receiver 'installer' | `cli/lucli/services/packages/PackagesMainCli.cfc:130` variable 'variables.installer' has no component ref |
| 8 | `variables.page → vendor/wheels/wheelstest/BrowserClient.cfc` | 1 | medium | declares click() | `vendor/wheels/wheelstest/BrowserClient.cfc:126` variable 'variables.page' has no component ref |
| 8 | `variables.registry → cli/lucli/services/packages/Registry.cfc` | 2 | low | declares listPackageNames(), fetchManifest(); named like the receiver 'registry' (2 candidates) | `cli/lucli/services/packages/PackagesMainCli.cfc:52` variable 'variables.registry' has no component ref |
| 7 | `cache → vendor/wheels/services/packages/ManifestCache.cfc` | 7 | low | declares refresh(), writeManifest(), hasFreshManifest() (2 candidates) | `vendor/wheels/tests/specs/packages/RegistryFetchManifestSpec.cfc:40` variable 'cache' has no component ref |
| 7 | `di → vendor/wheels/Injector.cfc` | 3 | low | declares containsInstance(), getInstance() (2 candidates) | `vendor/wheels/global/auth.cfm:40` variable 'di' has no component ref |
| 7 | `local.proc` | 0 | none |  | `cli/src/commands/wheels/db/dump.cfc:2010` variable 'local.proc' has no component ref |
| 7 | `local.value` | 0 | none |  | `vendor/wheels/view/formsdate.cfc:135` variable 'local.value' has no component ref |
| 7 | `migrator → vendor/wheels/Migrator.cfc` | 1 | high | declares getAvailableMigrations(), getCurrentMigrationVersion(); named like the receiver 'migrator' | `cli/lucli/services/MigrationRunner.cfc:46` variable 'migrator' has no component ref |
| 7 | `optlauncher → vendor/wheels/wheelstest/BrowserLauncher.cfc` | 2 | high | declares $classpathJarPaths(), resolveInstallDir(), $loadJars(), getState(), release(), $buildOption() | `vendor/wheels/tests/specs/wheelstest/BrowserLauncherSpec.cfc:341` variable 'optLauncher' has no component ref |
| 7 | `variables.$launcher → vendor/wheels/wheelstest/BrowserLauncher.cfc` | 1 | medium | declares $buildOption() | `vendor/wheels/wheelstest/BrowserClient.cfc:275` variable 'variables.$launcher' has no component ref |
| 7 | `variables.cache → cli/lucli/services/packages/ManifestCache.cfc` | 2 | low | declares hasFreshIndex(), readIndex(), writeIndex() (2 candidates) | `cli/lucli/services/packages/Registry.cfc:63` variable 'variables.cache' has no component ref |
| 7 | `variables.cache → vendor/wheels/services/packages/ManifestCache.cfc` | 2 | low | declares hasFreshIndex(), readIndex(), writeIndex() (2 candidates) | `vendor/wheels/services/packages/Registry.cfc:52` variable 'variables.cache' has no component ref |
| 7 | `variables.templateservice → cli/src/models/TemplateService.cfc` | 2 | medium | declares generateFromTemplate(); named like the receiver 'templateService' (2 candidates) | `cli/lucli/services/CodeGen.cfc:57` variable 'variables.templateService' has no component ref |
| 7 | `variables.wheels.class.adapter` | 3 | none |  | `vendor/wheels/Model.cfc:282` variable 'variables.wheels.class.adapter' has no component ref |
| 6 | `cache → cli/lucli/services/packages/ManifestCache.cfc` | 7 | low | declares refresh() (7 candidates) | `cli/lucli/tests/specs/packages/LegacyAdapterResolutionSpec.cfc:47` variable 'cache' has no component ref |
| 6 | `state.adapter` | 2 | none |  | `vendor/wheels/tests/specs/database/DatabaseAdapterHardenerSpec.cfc:208` variable 'state.adapter' has no component ref |
| 6 | `this.adapter → vendor/wheels/databaseAdapters/Abstract.cfc` | 2 | low | declares foreignKeySQL() (2 candidates) | `vendor/wheels/migrator/ForeignKeyDefinition.cfc:49` variable 'this.adapter' has no component ref |
| 6 | `variables.modelreference → vendor/wheels/model/query/ScopeChain.cfc` | 3 | low | declares findByKey() (2 candidates) | `vendor/wheels/model/query/ScopeChain.cfc:114` variable 'variables.modelReference' has no component ref |
| 5 | `adapter → vendor/wheels/databaseAdapters/Abstract.cfc` | 8 | low | declares quoteTableName() (8 candidates) | `vendor/wheels/migrator/Base.cfc:123` variable 'adapter' has no component ref |
| 5 | `arguments.role → cli/lucli/services/deploy/config/Role.cfc` | 2 | medium | declares cmd(); named like the receiver 'role' (2 candidates) | `cli/lucli/services/deploy/commands/AppCommands.cfc:28` variable 'arguments.role' has no component ref |
| 5 | `arguments.runner` | 1 | none |  | `vendor/wheels/wheelstest/system/BaseSpec.cfc:1026` variable 'arguments.runner' has no component ref |
| 5 | `ctx.g → vendor/wheels/Global.cfc` | 1 | medium | declares $includeConfig() | `vendor/wheels/tests/specs/global/includeConfigSpec.cfc:54` variable 'ctx.g' has no component ref |
| 5 | `ctx.s → cli/lucli/services/deploy/config/Ssh.cfc` | 1 | medium | declares $expandHome() | `cli/lucli/tests/specs/deploy/config/SshSpec.cfc:53` variable 'ctx.s' has no component ref |
| 5 | `loader → cli/lucli/services/deploy/lib/JarLoader.cfc` | 1 | medium | declares withIsolatedTCCL() | `cli/lucli/services/deploy/lib/Mustache.cfc:67` variable 'loader' has no component ref |
| 5 | `local.ctrl` | 1 | none |  | `vendor/wheels/Dispatch.cfc:440` variable 'local.ctrl' has no component ref |
| 5 | `local.instance` | 159 | none |  | `vendor/wheels/Test.cfc:550` variable 'local.instance' has no component ref |
| 5 | `local.migrator → vendor/wheels/Migrator.cfc` | 3 | high | declares migrateTo(), migrateToLatest(), redoMigration(), migrateIndividual(); named like the receiver 'migrator' | `vendor/wheels/public/migrator/command.cfm:107` variable 'local.migrator' has no component ref |
| 5 | `local.process` | 0 | none |  | `cli/src/commands/wheels/db/dump.cfc:1888` variable 'local.process' has no component ref |
| 5 | `local.sessionmanager → vendor/wheels/public/mcp/SessionManager.cfc` | 1 | high | declares updateSession(); named like the receiver 'sessionManager' | `vendor/wheels/public/mcp/McpServer.cfc:164` variable 'local.sessionManager' has no component ref |
| 5 | `pkg.controller → vendor/wheels/view/formsdate.cfc` | 1 | medium | declares $yearMonthHourMinuteSecondSelectTagContent() | `vendor/wheels/tests/specs/view/formsdateSpec.cfc:14` variable 'pkg.controller' has no component ref |
| 5 | `pub → vendor/wheels/Public.cfc` | 2 | low | declares $loadRegistryPackages() (2 candidates) | `vendor/wheels/tests/specs/packages/LoadRegistryPackagesSpec.cfc:36` variable 'pub' has no component ref |
| 5 | `r → vendor/wheels/services/packages/Registry.cfc` | 2 | low | declares fetchManifest(), listAll() (2 candidates) | `vendor/wheels/tests/specs/packages/RegistryFetchManifestSpec.cfc:33` variable 'r' has no component ref |
| 5 | `scaffold → cli/lucli/services/Scaffold.cfc` | 2 | medium | declares createMigrationWithProperties(); named like the receiver 'scaffold' (2 candidates) | `cli/lucli/Module.cfc:4821` variable 'scaffold' has no component ref |
| 5 | `svc → cli/lucli/services/Destroy.cfc` | 1 | high | declares previewDestroy(), destroyResource(), destroyModel(), destroyController(), destroyView() | `cli/lucli/Module.cfc:3277` variable 'svc' has no component ref |
| 5 | `t → vendor/wheels/migrator/TableDefinition.cfc` | 1 | medium | declares change() | `vendor/wheels/migrator/templates/change-table.cfc:20` variable 't' has no component ref |
| 5 | `variables.page → vendor/wheels/interfaces/StorageDiskInterface.cfc` | 3 | low | declares url() (3 candidates) | `vendor/wheels/wheelstest/BrowserClient.cfc:671` variable 'variables.page' has no component ref |
| 4 | `api → vendor/wheels/BuildInfo.cfc` | 6 | low | declares version() (6 candidates) | `vendor/wheels/tests/specs/mapperModernSpec.cfc:138` variable 'api' has no component ref |
| 4 | `application.wheels.dispatch → vendor/wheels/Dispatch.cfc` | 3 | medium | declares $request(); named like the receiver 'dispatch' (3 candidates) | `cli/lucli/templates/app/public/index.cfm:5` variable 'application.wheels.dispatch' has no component ref |
| 4 | `application.wirebox → vendor/wheels/Injector.cfc` | 3 | low | declares getInstance() (2 candidates) | `cli/src/commands/wheels/generate/app-wizard.cfc:76` variable 'application.wirebox' has no component ref |
| 4 | `arguments.applicationscope.wo` | 1 | none |  | `cli/lucli/templates/app/public/Application.cfc:233` variable 'arguments.applicationScope.wo' has no component ref |
| 4 | `arguments.applicationscope.wo → vendor/wheels/Global.cfc` | 3 | low | declares $include() (3 candidates) | `cli/lucli/templates/app/public/Application.cfc:198` variable 'arguments.applicationScope.wo' has no component ref |
| 4 | `arguments.container → vendor/wheels/Injector.cfc` | 3 | low | declares $snapshotBindings() (3 candidates) | `vendor/wheels/PackageLoader.cfc:1309` variable 'arguments.container' has no component ref |
| 4 | `arguments.dialog` | 0 | none |  | `vendor/wheels/wheelstest/DialogConsumer.cfc:24` variable 'arguments.dialog' has no component ref |
| 4 | `arguments.javasystem.out` | 3 | none |  | `cli/lucli/Module.cfc:2644` variable 'arguments.javaSystem.out' has no component ref |
| 4 | `arguments.migration.cfc → vendor/wheels/migrator/Migration.cfc` | 47 | low | declares down(), up() (47 candidates) | `vendor/wheels/Migrator.cfc:436` variable 'arguments.migration.cfc' has no component ref |
| 4 | `arguments.ownerclass → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/model/sql.cfm:1655` variable 'arguments.ownerClass' has no component ref |
| 4 | `arguments.target` | 159 | none |  | `vendor/wheels/wheelstest/system/runners/UnitRunner.cfc:73` variable 'arguments.target' has no component ref |
| 4 | `attributes.callbacks` | 0 | none |  | `vendor/wheels/wheelstest/system/runners/BDDRunner.cfc:259` variable 'attributes.callbacks' has no component ref |
| 4 | `b → cli/lucli/services/deploy/config/Builder.cfc` | 1 | high | declares dockerfile(), context() | `cli/lucli/services/deploy/commands/BuilderCommands.cfc:18` variable 'b' has no component ref |
| 4 | `cmds → cli/lucli/services/deploy/cli/DeployAppCli.cfc` | 4 | low | declares containers() (4 candidates) | `cli/lucli/services/deploy/cli/DeployAppCli.cfc:59` variable 'cmds' has no component ref |
| 4 | `command` | 0 | none |  | `cli/lucli/services/deploy/lib/SshClient.cfc:136` variable 'command' has no component ref |
| 4 | `limiter → tools/article-tests/Probe.cfc` | 16 | low | declares handle() (16 candidates) | `tools/article-tests/run.cfm:378` variable 'limiter' has no component ref |
| 4 | `loc → vendor/wheels/wheelstest/BrowserClient.cfc` | 1 | medium | declares waitFor() | `vendor/wheels/wheelstest/BrowserClient.cfc:297` variable 'loc' has no component ref |
| 4 | `local._model` | 1 | none |  | `vendor/wheels/model/serialize.cfm:59` variable 'local._model' has no component ref |
| 4 | `local.countrs` | 0 | none |  | `cli/src/commands/wheels/db/dump.cfc:736` variable 'local.countRs' has no component ref |
| 4 | `local.engine → vendor/wheels/Channel.cfc` | 1 | high | declares subscribe(), replay(), unsubscribe() | `vendor/wheels/controller/channels.cfc:233` variable 'local.engine' has no component ref |
| 4 | `local.env → vendor/wheels/interfaces/StorageDiskInterface.cfc` | 6 | low | declares put() (6 candidates) | `cli/src/commands/wheels/db/dump.cfc:453` variable 'local.env' has no component ref |
| 4 | `local.fknamesource → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/model/sql.cfm:1632` variable 'local.fkNameSource' has no component ref |
| 4 | `local.pb` | 0 | none |  | `cli/src/commands/wheels/db/shell.cfc:514` variable 'local.pb' has no component ref |
| 4 | `registry → cli/lucli/services/ServerRegistry.cfc` | 1 | high | declares serverNameFor(), inspect(), clean() | `cli/lucli/Module.cfc:1569` variable 'registry' has no component ref |
| 4 | `result.post → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 4 | low | declares key() (2 candidates) | `vendor/wheels/tests/specs/dispatch/routeModelBindingSpec.cfc:52` variable 'result.post' has no component ref |
| 4 | `s → cli/lucli/services/deploy/config/Ssh.cfc` | 1 | high | declares user(), port(), keys(), $expandHome() | `cli/lucli/services/deploy/lib/SshPoolFactory.cfc:45` variable 's' has no component ref |
| 4 | `thread.target → vendor/wheels/wheelstest/system/BaseSpec.cfc` | 2 | medium | declares runSpec() | `vendor/wheels/wheelstest/system/runners/BDDRunner.cfc:278` variable 'thread.target' has no component ref |
| 4 | `user.author → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/tests/specs/model/validationsSpec.cfc:26` variable 'user.author' has no component ref |
| 4 | `user.author → vendor/wheels/interfaces/model/ModelErrorInterface.cfc` | 2 | medium | declares addError() | `vendor/wheels/tests/_assets/controllers/ControllerWithNestedModelErrors.cfc:7` variable 'user.author' has no component ref |
| 4 | `user.author.profile → vendor/wheels/Model.cfc` | 2 | high | declares addError(), $classData() | `vendor/wheels/tests/specs/model/errorsSpec.cfc:24` variable 'user.author.profile' has no component ref |
| 4 | `variables.wheels.class.adapter → vendor/wheels/databaseAdapters/CockroachDB/CockroachDBModel.cfc` | 7 | low | declares $querySetup() (7 candidates) | `vendor/wheels/model/delete.cfm:198` variable 'variables.wheels.class.adapter' has no component ref |
| 4 | `variables.wheels.class.adapter → vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerModel.cfc` | 7 | low | declares $quoteIdentifier(), $getType() (5 candidates) | `vendor/wheels/model/sql.cfm:1085` variable 'variables.wheels.class.adapter' has no component ref |
| 3 | `application.wheels.mapper → vendor/wheels/Mapper.cfc` | 1 | high | declares $patternToRegex(); named like the receiver 'mapper' | `vendor/wheels/global/cors.cfm:213` variable 'application.wheels.mapper' has no component ref |
| 3 | `arguments.buffer` | 1 | none |  | `vendor/wheels/controller/channels.cfc:185` variable 'arguments.buffer' has no component ref |
| 3 | `arguments.columnowner → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/model/sql.cfm:1814` variable 'arguments.columnOwner' has no component ref |
| 3 | `arguments.container → vendor/wheels/tests/_assets/plugins/serviceprovider/FakeContainer.cfc` | 4 | low | declares asSingleton() (4 candidates) | `vendor/wheels/tests/_assets/plugins/serviceprovider/TestServiceProvider/TestServiceProvider.cfc:25` variable 'arguments.container' has no component ref |
| 3 | `arguments.modelinstance → vendor/wheels/Model.cfc` | 1 | high | declares $classData(), $expandedAssociationsMetadata() | `vendor/wheels/Seeder.cfc:480` variable 'arguments.modelInstance' has no component ref |
| 3 | `arguments.proc` | 0 | none |  | `cli/lucli/services/deploy/lib/SecretResolver.cfc:212` variable 'arguments.proc' has no component ref |
| 3 | `arguments.runner → vendor/wheels/wheelstest/system/TestBox.cfc` | 2 | high | declares announceToModules(); named like the receiver 'TestBox' | `vendor/wheels/wheelstest/system/BaseSpec.cfc:1062` variable 'arguments.runner' has no component ref |
| 3 | `arguments.runner → vendor/wheels/wheelstest/system/runners/BaseRunner.cfc` | 1 | high | declares canRunLabel(), canRunSpec() | `vendor/wheels/wheelstest/system/BaseSpec.cfc:1371` variable 'arguments.runner' has no component ref |
| 3 | `arguments.target → vendor/wheels/tests/specs/mapperModernSpec.cfc` | 706 | low | declares run(), beforeAll(), afterAll() (67 candidates) | `vendor/wheels/wheelstest/system/runners/BDDRunner.cfc:63` variable 'arguments.target' has no component ref |
| 3 | `arguments.writer → vendor/wheels/tests/_assets/channel/SseWriterFake.cfc` | 2 | low | declares flush() (2 candidates) | `vendor/wheels/controller/sse.cfc:109` variable 'arguments.writer' has no component ref |
| 3 | `auth` | 2 | none |  | `examples/starter-app/app/controllers/Sessions.cfc:24` variable 'auth' has no component ref |
| 3 | `cmds → cli/lucli/services/deploy/cli/DeployAccessoryCli.cfc` | 10 | low | declares start() (9 candidates) | `cli/lucli/services/deploy/cli/DeployAppCli.cfc:38` variable 'cmds' has no component ref |
| 3 | `compiler` | 0 | none |  | `cli/lucli/services/deploy/lib/Mustache.cfc:92` variable 'compiler' has no component ref |
| 3 | `fixtures.session.scaffold → cli/lucli/services/Scaffold.cfc` | 2 | medium | declares generateAuth(); named like the receiver 'scaffold' (2 candidates) | `cli/lucli/tests/specs/services/GenerateAuthSpec.cfc:412` variable 'fixtures.session.scaffold' has no component ref |
| 3 | `local.evalresult → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 2 | high | declares properties(), key(), isNew() | `vendor/wheels/public/views/consoleeval.cfm:233` variable 'local.evalResult' has no component ref |
| 3 | `local.jbcrypt` | 0 | none |  | `vendor/wheels/global/security.cfm:87` variable 'local.jbcrypt' has no component ref |
| 3 | `local.model` | 3 | none |  | `vendor/wheels/model/nestedproperties.cfm:302` variable 'local.model' has no component ref |
| 3 | `local.modelobj → vendor/wheels/Model.cfc` | 2 | high | declares tableName(), primaryKeys(), $classData() | `vendor/wheels/migrator/AutoMigrator.cfc:21` variable 'local.modelObj' has no component ref |
| 3 | `local.newrecord` | 2 | none |  | `vendor/wheels/Seeder.cfc:236` variable 'local.newRecord' has no component ref |
| 3 | `local.object` | 1 | none |  | `vendor/wheels/view/forms.cfc:399` variable 'local.object' has no component ref |
| 3 | `local.object → vendor/wheels/interfaces/model/ModelErrorInterface.cfc` | 2 | medium | declares allErrors() | `vendor/wheels/view/errors.cfc:26` variable 'local.object' has no component ref |
| 3 | `local.rv → vendor/wheels/wheelstest/system/MockBox.cfc` | 3 | medium | declares $callback() | `vendor/wheels/model/create.cfm:128` variable 'local.rv' has no component ref |
| 3 | `mapper` | 1 | none |  | `vendor/wheels/tests/specs/dispatch/routeModelBindingSpec.cfc:232` variable 'mapper' has no component ref |
| 3 | `methods[]` | 0 | none |  | `cli/lucli/services/deploy/lib/Mustache.cfc:75` variable 'methods[]' has no component ref |
| 3 | `miscellaneous → vendor/wheels/view/miscellaneous.cfc` | 1 | high | declares $getObject(); named like the receiver 'miscellaneous' | `vendor/wheels/tests/specs/view/miscellaneousSpec.cfc:36` variable 'miscellaneous' has no component ref |
| 3 | `modelinstance → vendor/wheels/Model.cfc` | 1 | high | declares $classData(), findOne(), primaryKeys() | `vendor/wheels/Seeder.cfc:321` variable 'modelInstance' has no component ref |
| 3 | `pkg.controller → vendor/wheels/view/formsdateplain.cfc` | 1 | medium | declares dateTimeSelectTags() | `vendor/wheels/tests/specs/view/formsdateplainSpec.cfc:67` variable 'pkg.controller' has no component ref |
| 3 | `settings.bcrypt` | 0 | none |  | `examples/starter-app/plugins/authenticateThis/authenticateThis.cfc:54` variable 'settings.bCrypt' has no component ref |
| 3 | `sftp` | 0 | none |  | `cli/lucli/services/deploy/lib/SshClient.cfc:285` variable 'sftp' has no component ref |
| 3 | `sh → vendor/wheels/middleware/SecurityHeaders.cfc` | 1 | medium | declares $headers() | `tools/article-tests/run.cfm:258` variable 'sh' has no component ref |
| 3 | `sock` | 0 | none |  | `cli/lucli/tests/StubHttpServer.cfc:45` variable 'sock' has no component ref |
| 3 | `sshjref` | 0 | none |  | `cli/lucli/services/deploy/lib/SshClient.cfc:129` variable 'sshjRef' has no component ref |
| 3 | `state.adapter → vendor/wheels/databaseAdapters/Abstract.cfc` | 4 | low | declares addColumnOptions() (4 candidates) | `vendor/wheels/tests/specs/migrator/addColumnOptionsSpec.cfc:35` variable 'state.adapter' has no component ref |
| 3 | `stdoutstream` | 0 | none |  | `cli/lucli/services/deploy/lib/SecretResolver.cfc:227` variable 'stdoutStream' has no component ref |
| 3 | `this.adapter → vendor/wheels/migrator/Migration.cfc` | 5 | low | declares dropTable(), createTable() (5 candidates) | `vendor/wheels/migrator/TableDefinition.cfc:396` variable 'this.adapter' has no component ref |
| 3 | `this.mockbox → vendor/wheels/wheelstest/system/MockBox.cfc` | 2 | high | declares normalizeArguments(); named like the receiver 'mockBox' | `public/testbox/system/stubs/F952D54F1096E25C030C8E3149ABD8C4.cfm:18` variable 'this.mockBox' has no component ref |
| 3 | `variables.$browser` | 0 | none |  | `vendor/wheels/wheelstest/BrowserTest.cfc:208` variable 'variables.$browser' has no component ref |
| 3 | `variables.$playwright` | 0 | none |  | `vendor/wheels/wheelstest/BrowserLauncher.cfc:673` variable 'variables.$playwright' has no component ref |
| 3 | `variables.context → vendor/wheels/wheelstest/BrowserClient.cfc` | 1 | medium | declares clearCookies() | `vendor/wheels/wheelstest/BrowserClient.cfc:502` variable 'variables.context' has no component ref |
| 3 | `variables.migration.adapter → vendor/wheels/databaseAdapters/Abstract.cfc` | 5 | low | declares createTable(), quoteTableName() (4 candidates) | `vendor/wheels/tests/specs/hardener/MigratorHardenerShouldSpec.cfc:451` variable 'variables.migration.adapter' has no component ref |
| 3 | `variables.page → vendor/wheels/wheelstest/TestClient.cfc` | 1 | medium | declares content() | `vendor/wheels/wheelstest/BrowserClient.cfc:609` variable 'variables.page' has no component ref |
| 3 | `variables.resolver → cli/lucli/services/packages/VersionResolver.cfc` | 1 | medium | declares compatibleVersions() | `cli/lucli/services/packages/PackagesMainCli.cfc:137` variable 'variables.resolver' has no component ref |
| 3 | `variables.semver → cli/lucli/services/SemVer.cfc` | 2 | low | declares satisfiesAll(), compare(); named like the receiver 'semver' (2 candidates) | `cli/lucli/services/packages/VersionResolver.cfc:57` variable 'variables.semver' has no component ref |
| 3 | `variables.sshpool → cli/lucli/services/deploy/lib/FakeSshPool.cfc` | 2 | low | declares $setSecretValues() (2 candidates) | `cli/lucli/services/deploy/cli/DeployAccessoryCli.cfc:147` variable 'variables.sshPool' has no component ref |
| 3 | `variables.wheels.class.adapter → vendor/wheels/databaseAdapters/MySQL/MySQLModel.cfc` | 5 | low | declares $defaultValues(), $querySetup(), $generatedKey() (3 candidates) | `vendor/wheels/model/create.cfm:305` variable 'variables.wheels.class.adapter' has no component ref |
| 3 | `variables.wheels.class.adapter → vendor/wheels/model/query/QueryBuilder.cfc` | 3 | low | declares $quoteValue() (3 candidates) | `vendor/wheels/model/onmissingmethod.cfm:250` variable 'variables.wheels.class.adapter' has no component ref |
| 2 | `adapter → vendor/wheels/migrator/ColumnDefinition.cfc` | 4 | low | declares addColumnOptions() (4 candidates) | `vendor/wheels/migrator/ColumnDefinition.cfc:59` variable 'adapter' has no component ref |
| 2 | `analysis → cli/lucli/services/Analysis.cfc` | 4 | medium | declares analyze(); named like the receiver 'analysis' (4 candidates) | `cli/lucli/Module.cfc:3072` variable 'analysis' has no component ref |
| 2 | `application.log → cli/src/models/DetailOutputService.cfc` | 2 | medium | declares error() | `examples/starter-app/app/jobs/ProcessOrdersJob.cfc:133` variable 'application.log' has no component ref |
| 2 | `arguments.app` | 0 | none |  | `vendor/wheels/tests/_assets/plugins/middleware/TestMiddlewarePluginA/TestMiddlewarePluginA.cfc:10` variable 'arguments.app' has no component ref |
| 2 | `arguments.array[] → vendor/wheels/Model.cfc` | 1 | high | declares $classData(), properties() | `vendor/wheels/controller/rendering.cfc:819` variable 'arguments.array[]' has no component ref |
| 2 | `arguments.collection → vendor/wheels/Policy.cfc` | 5 | low | declares whereIn() (5 candidates) | `vendor/wheels/controller/authorization.cfc:132` variable 'arguments.collection' has no component ref |
| 2 | `arguments.context.host → vendor/wheels/Public.cfc` | 1 | medium | declares $cliFormatMigrationStatus() | `vendor/wheels/public/CliBridge.cfc:355` variable 'arguments.context.host' has no component ref |
| 2 | `arguments.evalresult → vendor/wheels/interfaces/model/ModelErrorInterface.cfc` | 2 | high | declares hasErrors(), allErrors() | `vendor/wheels/public/views/consoleeval.cfm:255` variable 'arguments.evalResult' has no component ref |
| 2 | `arguments.foreignkey → vendor/wheels/migrator/ColumnDefinition.cfc` | 2 | low | declares toSQL() (2 candidates) | `vendor/wheels/databaseAdapters/Abstract.cfc:262` variable 'arguments.foreignKey' has no component ref |
| 2 | `arguments.klass` | 0 | none |  | `vendor/wheels/wheelstest/BrowserLauncher.cfc:615` variable 'arguments.klass' has no component ref |
| 2 | `arguments.node` | 0 | none |  | `cli/lucli/services/deploy/lib/Yaml.cfc:181` variable 'arguments.node' has no component ref |
| 2 | `arguments.reader.reader` | 0 | none |  | `cli/lucli/Module.cfc:2646` variable 'arguments.reader.reader' has no component ref |
| 2 | `arguments.spec` | 0 | none |  | `vendor/wheels/wheelstest/BrowserTest.cfc:171` variable 'arguments.spec' has no component ref |
| 2 | `arguments.suite → vendor/wheels/wheelstest/system/BaseSpec.cfc` | 2 | medium | declares beforeEach() | `vendor/wheels/wheelstest/system/BaseSpec.cfc:1192` variable 'arguments.suite' has no component ref |
| 2 | `authenticator → vendor/wheels/auth/Authenticator.cfc` | 2 | medium | declares hasStrategy(), registerStrategy(); named like the receiver 'authenticator' (2 candidates) | `vendor/wheels/global/auth.cfm:58` variable 'authenticator' has no component ref |
| 2 | `constructors[]` | 0 | none |  | `vendor/wheels/wheelstest/BrowserLauncher.cfc:617` variable 'constructors[]' has no component ref |
| 2 | `core → examples/starter-app/plugins/jsconfirm/JSConfirm.cfc` | 5 | low | declares linkTo() (5 candidates) | `examples/starter-app/plugins/jsconfirm/JSConfirm.cfc:13` variable 'core' has no component ref |
| 2 | `core → vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner01/Runner01.cfc` | 6 | low | declares URLFor() (5 candidates) | `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner01/Runner01.cfc:8` variable 'core' has no component ref |
| 2 | `core → vendor/wheels/tests/_assets/plugins/runner/runner01/Runner01.cfc` | 6 | low | declares URLFor() (5 candidates) | `vendor/wheels/tests/_assets/plugins/runner/runner01/Runner01.cfc:8` variable 'core' has no component ref |
| 2 | `ctors[]` | 0 | none |  | `cli/lucli/services/deploy/lib/JarLoader.cfc:111` variable 'ctors[]' has no component ref |
| 2 | `ctx.ctrl → vendor/wheels/controller/rendering.cfc` | 2 | low | declares renderText(), renderNothing() (2 candidates) | `vendor/wheels/tests/specs/controller/renderingSpec.cfc:872` variable 'ctx.ctrl' has no component ref |
| 2 | `env → cli/lucli/services/deploy/config/Env.cfc` | 2 | medium | declares secret(); named like the receiver 'env' (2 candidates) | `cli/lucli/services/deploy/commands/AccessoryCommands.cfc:104` variable 'env' has no component ref |
| 2 | `gallery.photos[] → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 2 | medium | declares isNew() | `vendor/wheels/tests/specs/model/hardener/ModelHardenerM2M8Spec.cfc:101` variable 'gallery.photos[]' has no component ref |
| 2 | `local.adapter → vendor/wheels/databaseAdapters/Base.cfc` | 8 | low | declares $acquireAdvisoryLock(), $releaseAdvisoryLock() (8 candidates) | `vendor/wheels/model/locking.cfm:32` variable 'local.adapter' has no component ref |
| 2 | `local.assoc → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 4 | low | declares key() (2 candidates) | `vendor/wheels/view/formsassociation.cfc:147` variable 'local.assoc' has no component ref |
| 2 | `local.auth → vendor/wheels/auth/Authenticator.cfc` | 1 | high | declares authenticateWith(), authenticate() | `vendor/wheels/middleware/AuthMiddleware.cfc:85` variable 'local.auth' has no component ref |
| 2 | `local.authenticator → vendor/wheels/auth/Authenticator.cfc` | 2 | high | declares getStrategyNames(), getStrategy(); named like the receiver 'authenticator' | `vendor/wheels/controller/authorization.cfc:264` variable 'local.authenticator' has no component ref |
| 2 | `local.bridge → vendor/wheels/public/CliBridge.cfc` | 1 | high | declares handles(), dispatch() | `vendor/wheels/public/views/cli.cfm:81` variable 'local.bridge' has no component ref |
| 2 | `local.col → vendor/wheels/migrator/ColumnDefinition.cfc` | 2 | low | declares toSQL() (2 candidates) | `vendor/wheels/databaseAdapters/Oracle/OracleMigrator.cfc:55` variable 'local.col' has no component ref |
| 2 | `local.entries` | 0 | none |  | `vendor/wheels/global/tags.cfm:629` variable 'local.entries' has no component ref |
| 2 | `local.entry.strategy → vendor/wheels/auth/AuthStrategy.cfc` | 12 | low | declares authenticate() (12 candidates) | `vendor/wheels/auth/Authenticator.cfc:149` variable 'local.entry.strategy' has no component ref |
| 2 | `local.errorstream` | 0 | none |  | `cli/src/commands/wheels/db/dump.cfc:1922` variable 'local.errorStream' has no component ref |
| 2 | `local.inputstream` | 0 | none |  | `cli/src/commands/wheels/db/dump.cfc:1909` variable 'local.inputStream' has no component ref |
| 2 | `local.proc → vendor/wheels/wheelstest/BrowserClient.cfc` | 1 | medium | declares waitFor() | `cli/src/commands/wheels/db/dump.cfc:465` variable 'local.proc' has no component ref |
| 2 | `local.process → vendor/wheels/wheelstest/BrowserClient.cfc` | 1 | medium | declares waitFor() | `cli/src/commands/wheels/db/restore.cfc:301` variable 'local.process' has no component ref |
| 2 | `local.rs2` | 0 | none |  | `cli/src/commands/wheels/db/dump.cfc:1636` variable 'local.rs2' has no component ref |
| 2 | `local.rv` | 1 | none |  | `vendor/wheels/model/create.cfm:67` variable 'local.rv' has no component ref |
| 2 | `local.writer → vendor/wheels/tests/_assets/channel/SseWriterFake.cfc` | 1 | medium | declares checkError() | `vendor/wheels/controller/channels.cfc:286` variable 'local.writer' has no component ref |
| 2 | `mapper2 → vendor/wheels/mapper/mapping.cfc` | 1 | medium | declares $draw() | `vendor/wheels/tests/specs/global/urlforSpec.cfc:248` variable 'mapper2' has no component ref |
| 2 | `migration.adapter → vendor/wheels/databaseAdapters/Abstract.cfc` | 7 | low | declares changeColumnInTable() (7 candidates) | `vendor/wheels/tests/specs/migrator/sqliteChangeColumnTransactionSpec.cfc:50` variable 'migration.adapter' has no component ref |
| 2 | `migration.cfc → vendor/wheels/migrator/Migration.cfc` | 47 | low | declares up(), down() (47 candidates) | `vendor/wheels/public/migrator/sql.cfm:23` variable 'migration.CFC' has no component ref |
| 2 | `nextclosure` | 0 | none |  | `vendor/wheels/wheelstest/system/BaseSpec.cfc:1290` variable 'nextClosure' has no component ref |
| 2 | `obj.gallery` | 2 | none |  | `vendor/wheels/tests/specs/model/callbacksSpec.cfc:466` variable 'obj.gallery' has no component ref |
| 2 | `permission → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 4 | low | declares key() (2 candidates) | `examples/starter-app/app/views/admin/permissions/_form.cfm:9` variable 'permission' has no component ref |
| 2 | `pipeline → vendor/wheels/middleware/Pipeline.cfc` | 706 | medium | declares run(); named like the receiver 'pipeline' (702 candidates) | `tools/article-tests/run.cfm:232` variable 'pipeline' has no component ref |
| 2 | `reg → cli/lucli/services/deploy/config/Registry.cfc` | 1 | high | declares server(), username() | `cli/lucli/services/deploy/commands/RegistryCommands.cfc:24` variable 'reg' has no component ref |
| 2 | `result.author → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 4 | low | declares key() (2 candidates) | `vendor/wheels/tests/specs/dispatch/routeModelBindingSpec.cfc:106` variable 'result.author' has no component ref |
| 2 | `results.author → vendor/wheels/migrator/TableDefinition.cfc` | 3 | low | declares primaryKey() (2 candidates) | `vendor/wheels/tests/specs/model/crudSpec.cfc:360` variable 'results.author' has no component ref |
| 2 | `results.shop → vendor/wheels/migrator/TableDefinition.cfc` | 3 | low | declares primaryKey() (2 candidates) | `vendor/wheels/tests/specs/model/crudSpec.cfc:372` variable 'results.shop' has no component ref |
| 2 | `s3down → vendor/wheels/interfaces/StorageDiskInterface.cfc` | 6 | low | declares put(), exists() (3 candidates) | `vendor/wheels/tests/specs/storage/StorageSpec.cfc:392` variable 's3down' has no component ref |
| 2 | `spec.$assert` | 0 | none |  | `vendor/wheels/tests/specs/wheelstest/BaseSpecDslAliasSpec.cfc:132` variable 'spec.$assert' has no component ref |
| 2 | `spec.browser → vendor/wheels/wheelstest/BrowserClient.cfc` | 1 | high | declares visitUrl(), assertSee() | `vendor/wheels/tests/specs/wheelstest/BrowserTestNotWiredSpec.cfc:15` variable 'spec.browser' has no component ref |
| 2 | `ssh → cli/lucli/tests/specs/deploy/lib/FakeSshPoolSpec.cfc` | 706 | low | declares run() (702 candidates) | `cli/lucli/tests/specs/deploy/lib/SshPoolSpec.cfc:21` variable 'ssh' has no component ref |
| 2 | `state.results → vendor/wheels/wheelstest/system/TestResult.cfc` | 2 | high | declares getTotalError(), getBundleStats() | `vendor/wheels/tests/specs/wheelstest/BDDRunnerErrorReportingSpec.cfc:23` variable 'state.results' has no component ref |
| 2 | `svc → cli/lucli/services/Stats.cfc` | 2 | low | declares getStats() (2 candidates) | `cli/lucli/Module.cfc:4081` variable 'svc' has no component ref |
| 2 | `testauthor.profile → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 4 | low | declares key() (2 candidates) | `vendor/wheels/tests/specs/model/nestedpropertiesSpec.cfc:383` variable 'testAuthor.profile' has no component ref |
| 2 | `variables.$class.plugins[] → vendor/wheels/ServiceProviderInterface.cfc` | 12 | low | declares register() (11 candidates) | `vendor/wheels/Plugins.cfc:647` variable 'variables.$class.plugins[]' has no component ref |
| 2 | `variables.$classloader → cli/lucli/services/deploy/lib/JarLoader.cfc` | 1 | medium | declares loadClass() | `vendor/wheels/wheelstest/BrowserLauncher.cfc:430` variable 'variables.$classLoader' has no component ref |
| 2 | `variables.context` | 0 | none |  | `vendor/wheels/wheelstest/BrowserClient.cfc:484` variable 'variables.context' has no component ref |
| 2 | `variables.sink → vendor/wheels/wheelstest/system/BaseSpec.cfc` | 2 | medium | declares println() | `cli/lucli/services/deploy/lib/Output.cfc:29` variable 'variables.sink' has no component ref |
| 2 | `variables.spec → vendor/wheels/wheelstest/system/BaseSpec.cfc` | 3 | low | declares expect() (2 candidates) | `vendor/wheels/wheelstest/system/CollectionExpectation.cfc:40` variable 'variables.spec' has no component ref |
| 2 | `variables.wheels.class.adapter → vendor/wheels/databaseAdapters/H2/H2Model.cfc` | 7 | low | declares $upsertSQL(), $querySetup() (6 candidates) | `vendor/wheels/model/bulk.cfm:134` variable 'variables.wheels.class.adapter' has no component ref |
| 2 | `variables.wheels.class.adapter → vendor/wheels/databaseAdapters/Oracle/OracleModel.cfc` | 2 | high | declares $bulkInsertSQL(), $querySetup() | `vendor/wheels/model/bulk.cfm:45` variable 'variables.wheels.class.adapter' has no component ref |
| 1 | `a → cli/lucli/services/deploy/config/Accessory.cfc` | 9 | low | declares name() (9 candidates) | `cli/lucli/tests/specs/deploy/config/AccessorySpec.cfc:15` variable 'a' has no component ref |
| 1 | `acc → cli/lucli/services/deploy/config/Env.cfc` | 2 | medium | declares secret(); named like the receiver 'env' (2 candidates) | `cli/lucli/services/deploy/cli/DeployAccessoryCli.cfc:77` variable 'acc' has no component ref |
| 1 | `adapter → cli/lucli/services/deploy/cli/DeploySecretsCli.cfc` | 8 | low | declares fetch() (8 candidates) | `cli/lucli/services/deploy/cli/DeploySecretsCli.cfc:51` variable 'adapter' has no component ref |
| 1 | `adapter → vendor/wheels/migrator/ForeignKeyDefinition.cfc` | 7 | low | declares addForeignKeyOptions() (7 candidates) | `vendor/wheels/migrator/ForeignKeyDefinition.cfc:69` variable 'adapter' has no component ref |
| 1 | `apimap → vendor/wheels/BuildInfo.cfc` | 6 | low | declares version() (6 candidates) | `vendor/wheels/tests/specs/mapper/MapperRobustnessSpec.cfc:171` variable 'apiMap' has no component ref |
| 1 | `application.$wheels.buildinfo → vendor/wheels/BuildInfo.cfc` | 6 | medium | declares version(); named like the receiver 'buildInfo' (6 candidates) | `vendor/wheels/events/onapplicationstart.cfc:88` variable 'application.$wheels.buildInfo' has no component ref |
| 1 | `application.$wheels.engineadapter → vendor/wheels/engineAdapters/Base.cfc` | 2 | low | declares prepareDIComplete() (2 candidates) | `vendor/wheels/events/onapplicationstart.cfc:431` variable 'application.$wheels.engineAdapter' has no component ref |
| 1 | `application.wheels.buildinfo → vendor/wheels/BuildInfo.cfc` | 6 | medium | declares version(); named like the receiver 'buildInfo' (6 candidates) | `vendor/wheels/tests/specs/events/frameworkVersionSpec.cfc:26` variable 'application.wheels.buildInfo' has no component ref |
| 1 | `application.wheels.controllers[] → vendor/wheels/Controller.cfc` | 1 | medium | declares $getControllerClassData() | `vendor/wheels/Controller.cfc:128` variable 'application.wheels.controllers[]' has no component ref |
| 1 | `application.wheels.models[] → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/Model.cfc:653` variable 'application.wheels.models[]' has no component ref |
| 1 | `application.wheels.public → vendor/wheels/Public.cfc` | 2 | medium | declares $loadRegistryPackages(); named like the receiver 'public' (2 candidates) | `vendor/wheels/public/views/packagelist.cfm:44` variable 'application.wheels.public' has no component ref |
| 1 | `application.wheels.public → vendor/wheels/tests/_assets/dispatch/InvokeMethodFixture.cfc` | 2 | low | declares getState() (2 candidates) | `vendor/wheels/tests/specs/dispatch/requestSpec.cfc:168` variable 'application.wheels.public' has no component ref |
| 1 | `application.wheels.seeder → vendor/wheels/Seeder.cfc` | 1 | high | declares hasSeedFiles(); named like the receiver 'seeder' | `vendor/wheels/public/CliBridge.cfc:893` variable 'application.wheels.seeder' has no component ref |
| 1 | `application.wheelsdi → vendor/wheels/Model.cfc` | 1 | medium | declares $initModelClass() | `vendor/wheels/tests/specs/model/validationsSpec.cfc:116` variable 'application.wheelsdi' has no component ref |
| 1 | `application[].mapper → vendor/wheels/mapper/mapping.cfc` | 1 | medium | declares $draw() | `vendor/wheels/global/routing.cfm:594` variable 'application[].mapper' has no component ref |
| 1 | `application[].pluginobj → vendor/wheels/PackageLoader.cfc` | 2 | low | declares getMethodProviders() (2 candidates) | `vendor/wheels/global/plugins.cfm:398` variable 'application[].PluginObj' has no component ref |
| 1 | `arguments.$partial → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/controller/rendering.cfc:538` variable 'arguments.$partial' has no component ref |
| 1 | `arguments.$partial[] → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/controller/rendering.cfc:541` variable 'arguments.$partial[]' has no component ref |
| 1 | `arguments.adapter → vendor/wheels/databaseAdapters/CockroachDB/CockroachDBMigrator.cfc` | 7 | low | declares adapterName() (7 candidates) | `vendor/wheels/migrator/TableDefinition.cfc:112` variable 'arguments.adapter' has no component ref |
| 1 | `arguments.applicationscope.$wheelsbrowserlauncher → cli/lucli/services/deploy/cli/DeployLockCli.cfc` | 3 | low | declares release() (3 candidates) | `cli/lucli/templates/app/public/Application.cfc:177` variable 'arguments.applicationScope.$wheelsBrowserLauncher' has no component ref |
| 1 | `arguments.applicationscope.$wheelsbrowserlauncher → vendor/wheels/wheelstest/BrowserLauncher.cfc` | 3 | low | declares release() (3 candidates) | `public/Application.cfc:177` variable 'arguments.applicationScope.$wheelsBrowserLauncher' has no component ref |
| 1 | `arguments.componentinstance` | 1 | none |  | `vendor/wheels/Mapper.cfc:444` variable 'arguments.componentInstance' has no component ref |
| 1 | `arguments.componentreference → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 2 | medium | declares primaryKeys() | `vendor/wheels/model/onmissingmethod.cfm:724` variable 'arguments.componentReference' has no component ref |
| 1 | `arguments.context.host` | 1 | none |  | `vendor/wheels/public/CliBridge.cfc:467` variable 'arguments.context.host' has no component ref |
| 1 | `arguments.javasystem → cli/lucli/Module.cfc` | 3 | low | declares console() (2 candidates) | `cli/lucli/Module.cfc:2609` variable 'arguments.javaSystem' has no component ref |
| 1 | `arguments.linereader` | 0 | none |  | `cli/lucli/Module.cfc:2695` variable 'arguments.lineReader' has no component ref |
| 1 | `arguments.mapper → vendor/wheels/Mapper.cfc` | 1 | high | declares $patternToRegex(); named like the receiver 'mapper' | `vendor/wheels/Dispatch.cfc:229` variable 'arguments.mapper' has no component ref |
| 1 | `arguments.millis` | 0 | none |  | `vendor/wheels/global/util.cfm:366` variable 'arguments.millis' has no component ref |
| 1 | `arguments.modelinstance` | 1 | none |  | `cli/src/models/AdminIntrospectionService.cfc:32` variable 'arguments.modelInstance' has no component ref |
| 1 | `arguments.obj → vendor/wheels/interfaces/model/ModelErrorInterface.cfc` | 2 | medium | declares errorsOn() | `vendor/wheels/tests/specs/model/validationsSpec.cfc:1301` variable 'arguments.obj' has no component ref |
| 1 | `arguments.object` | 1 | none |  | `vendor/wheels/model/miscellaneous.cfm:301` variable 'arguments.object' has no component ref |
| 1 | `arguments.object → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/controller/rendering.cfc:684` variable 'arguments.object' has no component ref |
| 1 | `arguments.plugin → vendor/wheels/tests/_assets/plugins/hardener_mutateapp/MutateAppPlugin/MutateAppPlugin.cfc` | 7 | low | declares onPluginLoad() (7 candidates) | `vendor/wheels/Plugins.cfc:857` variable 'arguments.plugin' has no component ref |
| 1 | `arguments.primarykeys[] → vendor/wheels/migrator/ColumnDefinition.cfc` | 1 | medium | declares toPrimaryKeySQL() | `vendor/wheels/databaseAdapters/Oracle/OracleMigrator.cfc:51` variable 'arguments.primaryKeys[]' has no component ref |
| 1 | `arguments.record → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/controller/authorization.cfc:185` variable 'arguments.record' has no component ref |
| 1 | `arguments.targetobject → vendor/wheels/wheelstest/system/mockutils/MockGenerator.cfc` | 3 | low | declares $include() (3 candidates) | `vendor/wheels/wheelstest/system/mockutils/MockGenerator.cfc:274` variable 'arguments.targetObject' has no component ref |
| 1 | `arguments.user → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 2 | medium | declares properties() | `examples/starter-app/app/global/auth.cfm:99` variable 'arguments.user' has no component ref |
| 1 | `arguments[] → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 2 | medium | declares properties() | `vendor/wheels/controller/rendering.cfc:687` variable 'arguments[]' has no component ref |
| 1 | `attributes.box.instance → vendor/wheels/Job.cfc` | 12 | low | declares perform() (12 candidates) | `vendor/wheels/Job.cfc:804` variable 'attributes.box.instance' has no component ref |
| 1 | `attributes.srv → vendor/wheels/wheelstest/DialogConsumer.cfc` | 1 | medium | declares accept() | `cli/lucli/tests/StubHttpServer.cfc:41` variable 'attributes.srv' has no component ref |
| 1 | `byname.api → cli/lucli/services/deploy/config/Role.cfc` | 1 | medium | declares runningProxy() | `cli/lucli/tests/specs/deploy/config/RoleSpec.cfc:42` variable 'byName.api' has no component ref |
| 1 | `byname.workers → cli/lucli/services/deploy/config/Role.cfc` | 1 | medium | declares runningProxy() | `cli/lucli/tests/specs/deploy/config/RoleSpec.cfc:43` variable 'byName.workers' has no component ref |
| 1 | `cached` | 0 | none |  | `vendor/wheels/wheelstest/BrowserLauncher.cfc:403` variable 'cached' has no component ref |
| 1 | `cmds → cli/lucli/services/TestRunner.cfc` | 706 | low | declares run() (702 candidates) | `cli/lucli/services/deploy/cli/DeployAppCli.cfc:31` variable 'cmds' has no component ref |
| 1 | `cmds → cli/lucli/services/deploy/cli/DeployLockCli.cfc` | 4 | low | declares status() (4 candidates) | `cli/lucli/services/deploy/cli/DeployAppCli.cfc:52` variable 'cmds' has no component ref |
| 1 | `config.moduleconfig → cli/src/ModuleConfig.cfc` | 1 | high | declares onUnload(); named like the receiver 'moduleConfig' | `vendor/wheels/wheelstest/system/TestBox.cfc:501` variable 'config.moduleConfig' has no component ref |
| 1 | `containerreceived → vendor/wheels/Injector.cfc` | 3 | low | declares getInstance() (2 candidates) | `vendor/wheels/tests/_assets/plugins/serviceprovider/TestServiceProvider/TestServiceProvider.cfc:36` variable 'containerReceived' has no component ref |
| 1 | `cookies → vendor/wheels/tests/_assets/channel/MidLoopPublishBuffer.cfc` | 1 | medium | declares size() | `vendor/wheels/wheelstest/BrowserClient.cfc:513` variable 'cookies' has no component ref |
| 1 | `core → examples/starter-app/plugins/FlashMessagesBootstrap/FlashMessagesBootstrap.cfc` | 2 | low | declares flashMessages() (2 candidates) | `examples/starter-app/plugins/FlashMessagesBootstrap/FlashMessagesBootstrap.cfc:10` variable 'core' has no component ref |
| 1 | `core → vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner02/Runner02.cfc` | 6 | low | declares URLFor() (5 candidates) | `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner02/Runner02.cfc:8` variable 'core' has no component ref |
| 1 | `core → vendor/wheels/tests/_assets/plugins/runner/runner02/Runner02.cfc` | 6 | low | declares URLFor() (5 candidates) | `vendor/wheels/tests/_assets/plugins/runner/runner02/Runner02.cfc:8` variable 'core' has no component ref |
| 1 | `ctx → vendor/wheels/Injector.cfc` | 3 | low | declares getInstance() (2 candidates) | `vendor/wheels/tests/specs/di/InjectorSpec.cfc:471` variable 'ctx' has no component ref |
| 1 | `ctx → vendor/wheels/public/mcp/SessionManager.cfc` | 2 | medium | declares getSession() | `vendor/wheels/tests/specs/hardener/AuthHardenerShouldSpec.cfc:360` variable 'ctx' has no component ref |
| 1 | `ctx.mw → vendor/wheels/middleware/AuthMiddleware.cfc` | 16 | low | declares handle() (16 candidates) | `vendor/wheels/middleware/Pipeline.cfc:60` variable 'ctx.mw' has no component ref |
| 1 | `dummycontroller → vendor/wheels/controller/csrf.cfc` | 1 | medium | declares $generateCookieAuthenticityToken() | `vendor/wheels/rocketunit_tests/env.cfm:58` variable 'dummyController' has no component ref |
| 1 | `e.cause` | 0 | none |  | `vendor/wheels/wheelstest/BrowserLauncher.cfc:455` variable 'e.cause' has no component ref |
| 1 | `generateseeder → vendor/wheels/Seeder.cfc` | 1 | medium | declares generateSeeds() | `vendor/wheels/public/CliBridge.cfc:920` variable 'generateSeeder' has no component ref |
| 1 | `i` | 1 | none |  | `vendor/wheels/tests/specs/model/miscellaneousSpec.cfc:42` variable 'i' has no component ref |
| 1 | `idata.type → vendor/wheels/wheelstest/system/reports/ANTJUnitReporter.cfc` | 4 | low | declares runReport() (4 candidates) | `vendor/wheels/wheelstest/system/TestBox.cfc:652` variable 'iData.type' has no component ref |
| 1 | `idx.i → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 4 | low | declares key() (2 candidates) | `vendor/wheels/tests/specs/model/nestedpropertiesSpec.cfc:149` variable 'idx.i' has no component ref |
| 1 | `innermap → vendor/wheels/wheelstest/TestClient.cfc` | 6 | low | declares put() (6 candidates) | `vendor/wheels/wheelstest/BrowserLauncher.cfc:175` variable 'innerMap' has no component ref |
| 1 | `item → vendor/wheels/wheelstest/system/BaseSpec.cfc` | 2 | medium | declares beforeEach() | `vendor/wheels/wheelstest/system/BaseSpec.cfc:1188` variable 'item' has no component ref |
| 1 | `keymaps` | 0 | none |  | `cli/lucli/Module.cfc:2697` variable 'keyMaps' has no component ref |
| 1 | `klass` | 0 | none |  | `cli/lucli/services/deploy/lib/JarLoader.cfc:109` variable 'klass' has no component ref |
| 1 | `launcher → vendor/wheels/tests/_assets/dispatch/InvokeMethodFixture.cfc` | 2 | low | declares getState() (2 candidates) | `vendor/wheels/tests/specs/wheelstest/BrowserTestLifecycleSpec.cfc:38` variable 'launcher' has no component ref |
| 1 | `local.associationmodel → vendor/wheels/interfaces/model/ModelErrorInterface.cfc` | 2 | medium | declares allErrors() | `vendor/wheels/model/errors.cfm:72` variable 'local.associationModel' has no component ref |
| 1 | `local.candidate.strategy → vendor/wheels/auth/AuthStrategy.cfc` | 8 | low | declares supports() (8 candidates) | `vendor/wheels/auth/Authenticator.cfc:118` variable 'local.candidate.strategy' has no component ref |
| 1 | `local.class → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/model/sql.cfm:1504` variable 'local.class' has no component ref |
| 1 | `local.classdata.adapter → vendor/wheels/model/query/QueryBuilder.cfc` | 3 | low | declares $quoteValue() (3 candidates) | `vendor/wheels/model/query/QueryBuilder.cfc:537` variable 'local.classData.adapter' has no component ref |
| 1 | `local.currentmodelobject` | 1 | none |  | `vendor/wheels/view/miscellaneous.cfc:624` variable 'local.currentModelObject' has no component ref |
| 1 | `local.dbadapter → vendor/wheels/channel/DatabaseAdapter.cfc` | 1 | medium | declares poll() | `vendor/wheels/controller/channels.cfc:327` variable 'local.dbAdapter' has no component ref |
| 1 | `local.defaultentry.strategy → vendor/wheels/auth/AuthStrategy.cfc` | 8 | low | declares supports() (8 candidates) | `vendor/wheels/auth/Authenticator.cfc:283` variable 'local.defaultEntry.strategy' has no component ref |
| 1 | `local.envmap` | 0 | none |  | `cli/src/commands/wheels/base.cfc:900` variable 'local.envMap' has no component ref |
| 1 | `local.fk → vendor/wheels/migrator/ForeignKeyDefinition.cfc` | 1 | medium | declares toForeignKeySQL() | `vendor/wheels/databaseAdapters/Oracle/OracleMigrator.cfc:71` variable 'local.fk' has no component ref |
| 1 | `local.intermediatemodel → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/model/sql.cfm:1351` variable 'local.intermediateModel' has no component ref |
| 1 | `local.model → vendor/wheels/migrator/TableDefinition.cfc` | 3 | low | declares primaryKey() (2 candidates) | `vendor/wheels/model/nestedproperties.cfm:199` variable 'local.model' has no component ref |
| 1 | `local.modelclass → vendor/wheels/interfaces/model/ModelFinderInterface.cfc` | 3 | low | declares findByKey() (2 candidates) | `vendor/wheels/Dispatch.cfc:797` variable 'local.modelClass' has no component ref |
| 1 | `local.modelinstance → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/public/CliBridge.cfc:468` variable 'local.modelInstance' has no component ref |
| 1 | `local.modelobj → vendor/wheels/interfaces/model/ModelFinderInterface.cfc` | 4 | low | declares findOne() (3 candidates) | `vendor/wheels/Seeder.cfc:218` variable 'local.modelObj' has no component ref |
| 1 | `local.modelobj → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 2 | medium | declares tableName() | `vendor/wheels/migrator/AutoMigrator.cfc:245` variable 'local.modelObj' has no component ref |
| 1 | `local.plugin → vendor/wheels/tests/_assets/plugins/hardener_mutateapp/MutateAppPlugin/MutateAppPlugin.cfc` | 5 | low | declares onPluginActivate() (5 candidates) | `vendor/wheels/Plugins.cfc:618` variable 'local.plugin' has no component ref |
| 1 | `local.policy → vendor/wheels/Policy.cfc` | 6 | medium | declares scope(); named like the receiver 'policy' (5 candidates) | `vendor/wheels/controller/authorization.cfc:134` variable 'local.policy' has no component ref |
| 1 | `local.reg → vendor/wheels/services/packages/Registry.cfc` | 3 | low | declares listAll() (3 candidates) | `vendor/wheels/Public.cfc:317` variable 'local.reg' has no component ref |
| 1 | `local.request` | 0 | none |  | `vendor/wheels/engineAdapters/BoxLang/BoxLangAdapter.cfc:43` variable 'local.request' has no component ref |
| 1 | `local.rv → vendor/wheels/Controller.cfc` | 1 | medium | declares $createControllerObject() | `vendor/wheels/global/objects.cfm:538` variable 'local.rv' has no component ref |
| 1 | `local.rv → vendor/wheels/interfaces/model/ModelPersistenceInterface.cfc` | 2 | medium | declares save() | `vendor/wheels/model/create.cfm:34` variable 'local.rv' has no component ref |
| 1 | `local.stmt2` | 0 | none |  | `cli/src/commands/wheels/db/dump.cfc:1634` variable 'local.stmt2' has no component ref |
| 1 | `local.strategy → vendor/wheels/Policy.cfc` | 3 | low | declares currentUser() (3 candidates) | `vendor/wheels/controller/authorization.cfc:268` variable 'local.strategy' has no component ref |
| 1 | `local.value → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 4 | low | declares key() (2 candidates) | `vendor/wheels/global/routing.cfm:422` variable 'local.value' has no component ref |
| 1 | `loctors[]` | 0 | none |  | `cli/lucli/services/deploy/lib/Yaml.cfc:112` variable 'loCtors[]' has no component ref |
| 1 | `map → vendor/wheels/mapper/scoping.cfc` | 3 | low | declares group() (3 candidates) | `vendor/wheels/tests/specs/mapperModernSpec.cfc:82` variable 'map' has no component ref |
| 1 | `mapfield` | 0 | none |  | `vendor/wheels/wheelstest/BrowserLauncher.cfc:172` variable 'mapField' has no component ref |
| 1 | `newrecord → vendor/wheels/interfaces/model/ModelPersistenceInterface.cfc` | 2 | medium | declares save() | `vendor/wheels/Seeder.cfc:357` variable 'newRecord' has no component ref |
| 1 | `obj.gallery.photos[] → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 2 | medium | declares properties() | `vendor/wheels/tests/specs/model/callbacksSpec.cfc:468` variable 'obj.gallery.photos[]' has no component ref |
| 1 | `omockgenerator → vendor/wheels/wheelstest/system/mockutils/MockGenerator.cfc` | 2 | medium | declares generate(); named like the receiver 'oMockGenerator' (2 candidates) | `vendor/wheels/wheelstest/system/MockBox.cfc:476` variable 'oMockGenerator' has no component ref |
| 1 | `os → cli/lucli/services/deploy/lib/Output.cfc` | 2 | low | declares flush() (2 candidates) | `cli/lucli/services/deploy/lib/SshClient.cfc:138` variable 'os' has no component ref |
| 1 | `outstream → cli/lucli/services/deploy/lib/Output.cfc` | 2 | low | declares flush() (2 candidates) | `cli/lucli/tests/StubHttpServer.cfc:59` variable 'outStream' has no component ref |
| 1 | `pagecontext → cli/src/models/AnalysisService.cfc` | 2 | medium | declares getConfig() | `tools/ci/setup-datasources.cfm:24` variable 'pageContext' has no component ref |
| 1 | `parent → vendor/wheels/Model.cfc` | 1 | medium | declares $classData() | `vendor/wheels/Seeder.cfc:509` variable 'parent' has no component ref |
| 1 | `parentsuite → vendor/wheels/wheelstest/system/BaseSpec.cfc` | 2 | medium | declares afterEach() | `vendor/wheels/wheelstest/system/BaseSpec.cfc:1335` variable 'parentSuite' has no component ref |
| 1 | `probe → cli/lucli/services/PortProbe.cfc` | 1 | medium | declares portInUse() | `cli/lucli/Module.cfc:8881` variable 'probe' has no component ref |
| 1 | `probes → tools/article-tests/Probes.cfc` | 1 | high | declares tryRateLimiterProxyStrategy(); named like the receiver 'probes' | `tools/article-tests/edge-cases.cfm:90` variable 'probes' has no component ref |
| 1 | `request.$wheelsdicompletelog[] → vendor/wheels/tests/_assets/di/LifecycleHookService.cfc` | 1 | medium | declares getCompleteCount() | `vendor/wheels/tests/specs/injector/InjectorHardenerSpec.cfc:348` variable 'request.$wheelsDICompleteLog[]' has no component ref |
| 1 | `request.wheels.toxml → vendor/wheels/wheelstest/system/util/XMLConverter.cfc` | 1 | medium | declares toXml() | `vendor/wheels/global/util.cfm:736` variable 'request.wheels.toXml' has no component ref |
| 1 | `runner → vendor/wheels/wheelstest/system/runners/BaseRunner.cfc` | 1 | medium | declares isSuiteFocused() | `vendor/wheels/wheelstest/system/BaseSpec.cfc:1013` variable 'runner' has no component ref |
| 1 | `scoped → vendor/wheels/wheelstest/BrowserClient.cfc` | 1 | medium | declares fill() | `vendor/wheels/tests/specs/wheelstest/BrowserIntegrationSpec.cfc:316` variable 'scoped' has no component ref |
| 1 | `seeder → vendor/wheels/Seeder.cfc` | 1 | high | declares runSeeds(); named like the receiver 'seeder' | `vendor/wheels/public/CliBridge.cfc:900` variable 'seeder' has no component ref |
| 1 | `server.system.out → vendor/wheels/wheelstest/system/BaseSpec.cfc` | 2 | medium | declares println() | `vendor/wheels/Migrator.cfc:1401` variable 'server.system.out' has no component ref |
| 1 | `serverservice` | 1 | none |  | `cli/src/models/BaseCommand.cfc:95` variable 'serverService' has no component ref |
| 1 | `sess → tools/lucee-extensions/sqlite/src/SQLite.cfc` | 1 | medium | declares getId() | `vendor/wheels/tests/specs/hardener/AuthHardenerShouldSpec.cfc:362` variable 'sess' has no component ref |
| 1 | `setter` | 0 | none |  | `vendor/wheels/wheelstest/BrowserLauncher.cfc:593` variable 'setter' has no component ref |
| 1 | `setting → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 4 | low | declares key() (2 candidates) | `examples/starter-app/app/views/admin/settings/edit.cfm:4` variable 'setting' has no component ref |
| 1 | `snippet → cli/lucli/Module.cfc` | 2 | low | declares generate() (2 candidates) | `cli/lucli/Module.cfc:5322` variable 'snippet' has no component ref |
| 1 | `svc` | 2 | none |  | `vendor/wheels/tests/specs/di/InjectorSpec.cfc:394` variable 'svc' has no component ref |
| 1 | `svc → cli/lucli/Module.cfc` | 2 | low | declares generateAdmin() (2 candidates) | `cli/lucli/Module.cfc:5389` variable 'svc' has no component ref |
| 1 | `svc → cli/lucli/services/Doctor.cfc` | 1 | medium | declares runChecks() | `cli/lucli/Module.cfc:3376` variable 'svc' has no component ref |
| 1 | `templates → cli/lucli/services/Templates.cfc` | 1 | high | declares getTemplateDir(); named like the receiver 'templates' | `cli/lucli/Module.cfc:5691` variable 'templates' has no component ref |
| 1 | `testablepackages` | 0 | none |  | `vendor/wheels/public/views/packages.cfm:53` variable 'testablePackages' has no component ref |
| 1 | `this.containerreceived → vendor/wheels/tests/_assets/plugins/serviceprovider/TrackingContainer.cfc` | 3 | low | declares containsInstance() (3 candidates) | `vendor/wheels/tests/_assets/plugins/serviceprovider/TestServiceProvider/TestServiceProvider.cfc:35` variable 'this.containerReceived' has no component ref |
| 1 | `this[] → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 2 | medium | declares setProperties() | `vendor/wheels/model/nestedproperties.cfm:176` variable 'this[]' has no component ref |
| 1 | `this[][]` | 1 | none |  | `vendor/wheels/model/nestedproperties.cfm:224` variable 'this[][]' has no component ref |
| 1 | `this[][] → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 2 | medium | declares setProperties() | `vendor/wheels/model/nestedproperties.cfm:279` variable 'this[][]' has no component ref |
| 1 | `user → vendor/wheels/interfaces/model/ModelPersistenceInterface.cfc` | 2 | medium | declares save() | `examples/starter-app/app/models/User.cfc:135` variable 'user' has no component ref |
| 1 | `user.author.profile → vendor/wheels/interfaces/model/ModelErrorInterface.cfc` | 2 | medium | declares addError() | `vendor/wheels/tests/_assets/controllers/ControllerWithNestedModelErrors.cfc:8` variable 'user.author.profile' has no component ref |
| 1 | `v1 → vendor/wheels/mapper/resources.cfc` | 2 | low | declares resources() (2 candidates) | `vendor/wheels/tests/specs/mapperModernSpec.cfc:171` variable 'v1' has no component ref |
| 1 | `variables.$context` | 0 | none |  | `vendor/wheels/wheelstest/BrowserTest.cfc:215` variable 'variables.$context' has no component ref |
| 1 | `variables.$scope` | 0 | none |  | `vendor/wheels/wheelstest/BrowserClient.cfc:856` variable 'variables.$scope' has no component ref |
| 1 | `variables.config → cli/lucli/services/deploy/config/Proxy.cfc` | 1 | high | declares healthcheck(); named like the receiver 'proxy' | `cli/lucli/services/deploy/commands/ProxyCommands.cfc:36` variable 'variables.config' has no component ref |
| 1 | `variables.config → cli/lucli/services/deploy/config/Registry.cfc` | 1 | high | declares server(); named like the receiver 'registry' | `cli/lucli/services/deploy/commands/RegistryCommands.cfc:31` variable 'variables.config' has no component ref |
| 1 | `variables.config → cli/lucli/services/deploy/config/Ssh.cfc` | 1 | high | declares user(); named like the receiver 'ssh' | `cli/lucli/services/deploy/commands/ProxyCommands.cfc:102` variable 'variables.config' has no component ref |
| 1 | `variables.fkadapter → vendor/wheels/databaseAdapters/Abstract.cfc` | 5 | low | declares createTable() (5 candidates) | `vendor/wheels/tests/specs/hardener/MigratorHardenerSpec.cfc:339` variable 'variables.fkAdapter' has no component ref |
| 1 | `variables.http → cli/lucli/services/packages/HttpClient.cfc` | 3 | low | declares download() (3 candidates) | `cli/lucli/services/packages/Installer.cfc:96` variable 'variables.http' has no component ref |
| 1 | `variables.jwtservice → vendor/wheels/auth/JwtService.cfc` | 1 | high | declares decode(); named like the receiver 'jwtService' | `vendor/wheels/auth/JwtStrategy.cfc:70` variable 'variables.jwtService' has no component ref |
| 1 | `variables.migration.adapter → vendor/wheels/databaseAdapters/CockroachDB/CockroachDBMigrator.cfc` | 7 | low | declares adapterName() (7 candidates) | `vendor/wheels/tests/specs/hardener/MigratorHardenerShouldSpec.cfc:26` variable 'variables.migration.adapter' has no component ref |
| 1 | `variables.modelreference` | 1 | none |  | `vendor/wheels/model/query/ScopeChain.cfc:46` variable 'variables.modelReference' has no component ref |
| 1 | `variables.page → vendor/wheels/interfaces/model/ModelFinderInterface.cfc` | 4 | low | declares reload() (2 candidates) | `vendor/wheels/wheelstest/BrowserClient.cfc:60` variable 'variables.page' has no component ref |
| 1 | `variables.pool → cli/lucli/services/deploy/lib/SshPool.cfc` | 1 | medium | declares getConnection() | `cli/lucli/services/deploy/lib/SshPoolTask.cfc:25` variable 'variables.pool' has no component ref |
| 1 | `variables.securerandom` | 0 | none |  | `vendor/wheels/auth/PasswordHasher.cfc:349` variable 'variables.secureRandom' has no component ref |
| 1 | `variables.simpleservice → vendor/wheels/tests/_assets/di/SimpleService.cfc` | 2 | medium | declares greet(); named like the receiver 'simpleService' (2 candidates) | `vendor/wheels/tests/_assets/di/DependentService.cfc:17` variable 'variables.simpleService' has no component ref |
| 1 | `yamlctors[]` | 0 | none |  | `cli/lucli/services/deploy/lib/Yaml.cfc:150` variable 'yamlCtors[]' has no component ref |

<details><summary>Groups with several candidates</summary>

- `variables.helpers → cli/lucli/services/Helpers.cfc` — 66 finding(s), 3 candidate(s):
  - low `cli/lucli/services/Helpers.cfc` — declares pluralize(), capitalize(); named like the receiver 'helpers'
  - low `cli/src/models/helpers.cfc` — declares pluralize(), capitalize(); named like the receiver 'helpers'
  - low `cli/src/models/CodeGenerationService.cfc` — declares pluralize(), capitalize()
- `application.wheels.engineadapter → vendor/wheels/engineAdapters/Base.cfc` — 55 finding(s), 2 candidate(s):
  - low `vendor/wheels/engineAdapters/Base.cfc` — declares parseFormKey()
  - low `vendor/wheels/engineAdapters/BoxLang/BoxLangAdapter.cfc` — declares parseFormKey()
- `migration.adapter → vendor/wheels/databaseAdapters/CockroachDB/CockroachDBMigrator.cfc` — 46 finding(s), 7 candidate(s):
  - low `vendor/wheels/databaseAdapters/CockroachDB/CockroachDBMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/H2/H2Migrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/MySQL/MySQLMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/SQLite/SQLiteMigrator.cfc` — declares adapterName()
- `rl → tools/article-tests/Probe.cfc` — 36 finding(s), 16 candidate(s):
  - low `tools/article-tests/Probe.cfc` — declares handle()
  - low `vendor/wheels/middleware/AuthMiddleware.cfc` — declares handle()
  - low `vendor/wheels/middleware/BrowserTestFixtureGuard.cfc` — declares handle()
  - low `vendor/wheels/middleware/Cors.cfc` — declares handle()
  - low `vendor/wheels/middleware/MiddlewareInterface.cfc` — declares handle()
  - low `vendor/wheels/middleware/RateLimiter.cfc` — declares handle()
  - low `vendor/wheels/middleware/RequestId.cfc` — declares handle()
  - low `vendor/wheels/middleware/SecurityHeaders.cfc` — declares handle()
  - … 8 more
- `variables.wheels.class.adapter → vendor/wheels/databaseAdapters/Base.cfc` — 26 finding(s), 2 candidate(s):
  - low `vendor/wheels/databaseAdapters/Base.cfc` — declares $setSharedModel(), $getColumns()
  - low `vendor/wheels/interfaces/database/DatabaseModelAdapterInterface.cfc` — declares $setSharedModel(), $getColumns()
- `adapter → vendor/wheels/engineAdapters/Base.cfc` — 24 finding(s), 2 candidate(s):
  - low `vendor/wheels/engineAdapters/Base.cfc` — declares invokeMethod()
  - low `vendor/wheels/engineAdapters/BoxLang/BoxLangAdapter.cfc` — declares invokeMethod()
- `variables.sshpool → cli/lucli/services/deploy/lib/SshPool.cfc` — 20 finding(s), 2 candidate(s):
  - medium `cli/lucli/services/deploy/lib/SshPool.cfc` — declares onEach(); named like the receiver 'sshPool'
  - low `cli/lucli/services/deploy/lib/FakeSshPool.cfc` — declares onEach()
- `arguments.injector → vendor/wheels/Injector.cfc` — 19 finding(s), 4 candidate(s):
  - low `vendor/wheels/Injector.cfc` — declares to()
  - low `vendor/wheels/interfaces/di/InjectorInterface.cfc` — declares to()
  - low `vendor/wheels/tests/_assets/plugins/serviceprovider/FakeContainer.cfc` — declares to()
  - low `vendor/wheels/tests/_assets/plugins/serviceprovider/TrackingContainer.cfc` — declares to()
- `role → cli/lucli/services/deploy/config/Role.cfc` — 18 finding(s), 2 candidate(s):
  - medium `cli/lucli/services/deploy/config/Role.cfc` — declares name(), hosts(); named like the receiver 'role'
  - low `cli/lucli/services/deploy/config/Accessory.cfc` — declares name(), hosts()
- `ssh → cli/lucli/services/TestRunner.cfc` — 17 finding(s), 702 candidate(s):
  - low `cli/lucli/services/TestRunner.cfc` — declares run()
  - low `cli/lucli/services/deploy/commands/AccessoryCommands.cfc` — declares run()
  - low `cli/lucli/services/deploy/commands/AppCommands.cfc` — declares run()
  - low `cli/lucli/services/deploy/lib/SshClient.cfc` — declares run()
  - low `cli/lucli/tests/specs/commands/CliHardenerS1S10Spec.cfc` — declares run()
  - low `cli/lucli/tests/specs/commands/CommandArgParsingSpec.cfc` — declares run()
  - low `cli/lucli/tests/specs/commands/ConsoleCommandSpec.cfc` — declares run()
  - low `cli/lucli/tests/specs/commands/CreateCommandSpec.cfc` — declares run()
  - … 694 more
- `binder → vendor/wheels/Injector.cfc` — 16 finding(s), 4 candidate(s):
  - low `vendor/wheels/Injector.cfc` — declares to()
  - low `vendor/wheels/interfaces/di/InjectorInterface.cfc` — declares to()
  - low `vendor/wheels/tests/_assets/plugins/serviceprovider/FakeContainer.cfc` — declares to()
  - low `vendor/wheels/tests/_assets/plugins/serviceprovider/TrackingContainer.cfc` — declares to()
- `application.wheelsdi → vendor/wheels/Injector.cfc` — 15 finding(s), 2 candidate(s):
  - low `vendor/wheels/Injector.cfc` — declares getInstance()
  - low `vendor/wheels/interfaces/di/InjectorInterface.cfc` — declares getInstance()
- `arguments.context.migrator → vendor/wheels/Migrator.cfc` — 14 finding(s), 3 candidate(s):
  - medium `vendor/wheels/Migrator.cfc` — declares createMigration(); named like the receiver 'migrator'
  - low `vendor/wheels/public/CliBridge.cfc` — declares createMigration()
  - low `cli/src/models/MigrationService.cfc` — declares createMigration()
- `stack.cache → cli/lucli/services/packages/ManifestCache.cfc` — 12 finding(s), 7 candidate(s):
  - low `cli/lucli/services/packages/ManifestCache.cfc` — declares refresh()
  - low `cli/lucli/services/packages/PackagesRegistryCli.cfc` — declares refresh()
  - low `cli/lucli/services/packages/Registry.cfc` — declares refresh()
  - low `vendor/wheels/auth/JwtService.cfc` — declares refresh()
  - low `vendor/wheels/wheelstest/BrowserClient.cfc` — declares refresh()
  - low `vendor/wheels/services/packages/ManifestCache.cfc` — declares refresh()
  - low `vendor/wheels/services/packages/Registry.cfc` — declares refresh()
- `variables.modelreference → vendor/wheels/model/query/QueryBuilder.cfc` — 12 finding(s), 3 candidate(s):
  - low `vendor/wheels/model/query/QueryBuilder.cfc` — declares findOne()
  - low `vendor/wheels/model/query/ScopeChain.cfc` — declares findOne()
  - low `vendor/wheels/interfaces/model/ModelFinderInterface.cfc` — declares findOne()
- `acc → cli/lucli/services/deploy/config/Accessory.cfc` — 11 finding(s), 2 candidate(s):
  - low `cli/lucli/services/deploy/config/Accessory.cfc` — declares env(), hosts(), name()
  - low `cli/lucli/services/deploy/config/Role.cfc` — declares env(), hosts(), name()
- `map → vendor/wheels/mapper/resources.cfc` — 11 finding(s), 2 candidate(s):
  - low `vendor/wheels/mapper/resources.cfc` — declares resources(), resource()
  - low `vendor/wheels/interfaces/routing/RouteMapperInterface.cfc` — declares resources(), resource()
- `codegen → cli/lucli/services/CodeGen.cfc` — 10 finding(s), 2 candidate(s):
  - medium `cli/lucli/services/CodeGen.cfc` — declares validateName(), generateModel(); named like the receiver 'codegen'
  - low `cli/src/models/CodeGenerationService.cfc` — declares validateName(), generateModel()
- `application.wheels.migrator → vendor/wheels/Migrator.cfc` — 9 finding(s), 2 candidate(s):
  - medium `vendor/wheels/Migrator.cfc` — declares migrateToLatest(); named like the receiver 'migrator'
  - low `vendor/wheels/public/CliBridge.cfc` — declares migrateToLatest()
- `arguments.column → vendor/wheels/migrator/ColumnDefinition.cfc` — 9 finding(s), 2 candidate(s):
  - low `vendor/wheels/migrator/ColumnDefinition.cfc` — declares toSQL()
  - low `vendor/wheels/migrator/ForeignKeyDefinition.cfc` — declares toSQL()
- `variables.codegenservice → cli/lucli/services/CodeGen.cfc` — 9 finding(s), 3 candidate(s):
  - low `cli/lucli/services/CodeGen.cfc` — declares generateModel(), generateController(), generateView(), generateTest()
  - low `cli/lucli/Module.cfc` — declares generateModel(), generateController(), generateView(), generateTest()
  - low `cli/src/models/CodeGenerationService.cfc` — declares generateModel(), generateController(), generateView(), generateTest()
- `s3 → vendor/wheels/interfaces/StorageDiskInterface.cfc` — 8 finding(s), 3 candidate(s):
  - low `vendor/wheels/interfaces/StorageDiskInterface.cfc` — declares url(), signedUrl()
  - low `vendor/wheels/storage/drivers/LocalDisk.cfc` — declares url(), signedUrl()
  - low `vendor/wheels/storage/drivers/S3Disk.cfc` — declares url(), signedUrl()
- `variables.registry → cli/lucli/services/packages/Registry.cfc` — 8 finding(s), 2 candidate(s):
  - low `cli/lucli/services/packages/Registry.cfc` — declares listPackageNames(), fetchManifest(); named like the receiver 'registry'
  - low `vendor/wheels/services/packages/Registry.cfc` — declares listPackageNames(), fetchManifest(); named like the receiver 'registry'
- `cache → vendor/wheels/services/packages/ManifestCache.cfc` — 7 finding(s), 2 candidate(s):
  - low `vendor/wheels/services/packages/ManifestCache.cfc` — declares refresh(), writeManifest(), hasFreshManifest()
  - low `cli/lucli/services/packages/ManifestCache.cfc` — declares refresh(), writeManifest(), hasFreshManifest()
- `di → vendor/wheels/Injector.cfc` — 7 finding(s), 2 candidate(s):
  - low `vendor/wheels/Injector.cfc` — declares containsInstance(), getInstance()
  - low `vendor/wheels/interfaces/di/InjectorInterface.cfc` — declares containsInstance(), getInstance()
- `variables.cache → cli/lucli/services/packages/ManifestCache.cfc` — 7 finding(s), 2 candidate(s):
  - low `cli/lucli/services/packages/ManifestCache.cfc` — declares hasFreshIndex(), readIndex(), writeIndex()
  - low `vendor/wheels/services/packages/ManifestCache.cfc` — declares hasFreshIndex(), readIndex(), writeIndex()
- `variables.cache → vendor/wheels/services/packages/ManifestCache.cfc` — 7 finding(s), 2 candidate(s):
  - low `vendor/wheels/services/packages/ManifestCache.cfc` — declares hasFreshIndex(), readIndex(), writeIndex()
  - low `cli/lucli/services/packages/ManifestCache.cfc` — declares hasFreshIndex(), readIndex(), writeIndex()
- `variables.templateservice → cli/src/models/TemplateService.cfc` — 7 finding(s), 2 candidate(s):
  - medium `cli/src/models/TemplateService.cfc` — declares generateFromTemplate(); named like the receiver 'templateService'
  - low `cli/lucli/services/Templates.cfc` — declares generateFromTemplate()
- `cache → cli/lucli/services/packages/ManifestCache.cfc` — 6 finding(s), 7 candidate(s):
  - low `cli/lucli/services/packages/ManifestCache.cfc` — declares refresh()
  - low `cli/lucli/services/packages/PackagesRegistryCli.cfc` — declares refresh()
  - low `cli/lucli/services/packages/Registry.cfc` — declares refresh()
  - low `vendor/wheels/auth/JwtService.cfc` — declares refresh()
  - low `vendor/wheels/wheelstest/BrowserClient.cfc` — declares refresh()
  - low `vendor/wheels/services/packages/ManifestCache.cfc` — declares refresh()
  - low `vendor/wheels/services/packages/Registry.cfc` — declares refresh()
- `this.adapter → vendor/wheels/databaseAdapters/Abstract.cfc` — 6 finding(s), 2 candidate(s):
  - low `vendor/wheels/databaseAdapters/Abstract.cfc` — declares foreignKeySQL()
  - low `vendor/wheels/interfaces/database/DatabaseMigratorAdapterInterface.cfc` — declares foreignKeySQL()
- `variables.modelreference → vendor/wheels/model/query/ScopeChain.cfc` — 6 finding(s), 2 candidate(s):
  - low `vendor/wheels/model/query/ScopeChain.cfc` — declares findByKey()
  - low `vendor/wheels/interfaces/model/ModelFinderInterface.cfc` — declares findByKey()
- `adapter → vendor/wheels/databaseAdapters/Abstract.cfc` — 5 finding(s), 8 candidate(s):
  - low `vendor/wheels/databaseAdapters/Abstract.cfc` — declares quoteTableName()
  - low `vendor/wheels/databaseAdapters/H2/H2Migrator.cfc` — declares quoteTableName()
  - low `vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerMigrator.cfc` — declares quoteTableName()
  - low `vendor/wheels/databaseAdapters/MySQL/MySQLMigrator.cfc` — declares quoteTableName()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleMigrator.cfc` — declares quoteTableName()
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLMigrator.cfc` — declares quoteTableName()
  - low `vendor/wheels/databaseAdapters/SQLite/SQLiteMigrator.cfc` — declares quoteTableName()
  - low `vendor/wheels/interfaces/database/DatabaseMigratorAdapterInterface.cfc` — declares quoteTableName()
- `arguments.role → cli/lucli/services/deploy/config/Role.cfc` — 5 finding(s), 2 candidate(s):
  - medium `cli/lucli/services/deploy/config/Role.cfc` — declares cmd(); named like the receiver 'role'
  - low `cli/lucli/services/deploy/config/Accessory.cfc` — declares cmd()
- `pub → vendor/wheels/Public.cfc` — 5 finding(s), 2 candidate(s):
  - low `vendor/wheels/Public.cfc` — declares $loadRegistryPackages()
  - low `vendor/wheels/tests/_assets/packages/FakePublic.cfc` — declares $loadRegistryPackages()
- `r → vendor/wheels/services/packages/Registry.cfc` — 5 finding(s), 2 candidate(s):
  - low `vendor/wheels/services/packages/Registry.cfc` — declares fetchManifest(), listAll()
  - low `cli/lucli/services/packages/Registry.cfc` — declares fetchManifest(), listAll()
- `scaffold → cli/lucli/services/Scaffold.cfc` — 5 finding(s), 2 candidate(s):
  - medium `cli/lucli/services/Scaffold.cfc` — declares createMigrationWithProperties(); named like the receiver 'scaffold'
  - low `cli/src/models/ScaffoldService.cfc` — declares createMigrationWithProperties()
- `variables.page → vendor/wheels/interfaces/StorageDiskInterface.cfc` — 5 finding(s), 3 candidate(s):
  - low `vendor/wheels/interfaces/StorageDiskInterface.cfc` — declares url()
  - low `vendor/wheels/storage/drivers/LocalDisk.cfc` — declares url()
  - low `vendor/wheels/storage/drivers/S3Disk.cfc` — declares url()
- `api → vendor/wheels/BuildInfo.cfc` — 4 finding(s), 6 candidate(s):
  - low `vendor/wheels/BuildInfo.cfc` — declares version()
  - low `vendor/wheels/mapper/scoping.cfc` — declares version()
  - low `vendor/wheels/interfaces/routing/RouteMapperInterface.cfc` — declares version()
  - low `cli/lucli/Module.cfc` — declares version()
  - low `cli/lucli/tests/_modules/BaseModule.cfc` — declares version()
  - low `cli/lucli/services/deploy/cli/DeployMainCli.cfc` — declares version()
- `application.wheels.dispatch → vendor/wheels/Dispatch.cfc` — 4 finding(s), 3 candidate(s):
  - medium `vendor/wheels/Dispatch.cfc` — declares $request(); named like the receiver 'dispatch'
  - low `vendor/wheels/storage/drivers/S3Disk.cfc` — declares $request()
  - low `vendor/wheels/tests/_assets/storage/S3DiskDeleteStub.cfc` — declares $request()
- `application.wirebox → vendor/wheels/Injector.cfc` — 4 finding(s), 2 candidate(s):
  - low `vendor/wheels/Injector.cfc` — declares getInstance()
  - low `vendor/wheels/interfaces/di/InjectorInterface.cfc` — declares getInstance()
- `arguments.applicationscope.wo → vendor/wheels/Global.cfc` — 4 finding(s), 3 candidate(s):
  - low `vendor/wheels/Global.cfc` — declares $include()
  - low `vendor/wheels/tests/_assets/events/CorsArbitrationEventDouble.cfc` — declares $include()
  - low `vendor/wheels/wheelstest/system/mockutils/MockGenerator.cfc` — declares $include()
- `arguments.container → vendor/wheels/Injector.cfc` — 4 finding(s), 3 candidate(s):
  - low `vendor/wheels/Injector.cfc` — declares $snapshotBindings()
  - low `vendor/wheels/tests/_assets/plugins/serviceprovider/FakeContainer.cfc` — declares $snapshotBindings()
  - low `vendor/wheels/tests/_assets/plugins/serviceprovider/TrackingContainer.cfc` — declares $snapshotBindings()
- `arguments.migration.cfc → vendor/wheels/migrator/Migration.cfc` — 4 finding(s), 47 candidate(s):
  - low `vendor/wheels/migrator/Migration.cfc` — declares down(), up()
  - low `vendor/wheels/migrator/templates/announce.cfc` — declares down(), up()
  - low `vendor/wheels/migrator/templates/blank.cfc` — declares down(), up()
  - low `vendor/wheels/migrator/templates/change-column.cfc` — declares down(), up()
  - low `vendor/wheels/migrator/templates/change-table.cfc` — declares down(), up()
  - low `vendor/wheels/migrator/templates/create-column.cfc` — declares down(), up()
  - low `vendor/wheels/migrator/templates/create-index.cfc` — declares down(), up()
  - low `vendor/wheels/migrator/templates/create-record.cfc` — declares down(), up()
  - … 39 more
- `cmds → cli/lucli/services/deploy/cli/DeployAppCli.cfc` — 4 finding(s), 4 candidate(s):
  - low `cli/lucli/services/deploy/cli/DeployAppCli.cfc` — declares containers()
  - low `cli/lucli/services/deploy/cli/DeployPruneCli.cfc` — declares containers()
  - low `cli/lucli/services/deploy/commands/AppCommands.cfc` — declares containers()
  - low `cli/lucli/services/deploy/commands/PruneCommands.cfc` — declares containers()
- `limiter → tools/article-tests/Probe.cfc` — 4 finding(s), 16 candidate(s):
  - low `tools/article-tests/Probe.cfc` — declares handle()
  - low `vendor/wheels/middleware/AuthMiddleware.cfc` — declares handle()
  - low `vendor/wheels/middleware/BrowserTestFixtureGuard.cfc` — declares handle()
  - low `vendor/wheels/middleware/Cors.cfc` — declares handle()
  - low `vendor/wheels/middleware/MiddlewareInterface.cfc` — declares handle()
  - low `vendor/wheels/middleware/RateLimiter.cfc` — declares handle()
  - low `vendor/wheels/middleware/RequestId.cfc` — declares handle()
  - low `vendor/wheels/middleware/SecurityHeaders.cfc` — declares handle()
  - … 8 more
- `local.env → vendor/wheels/interfaces/StorageDiskInterface.cfc` — 4 finding(s), 6 candidate(s):
  - low `vendor/wheels/interfaces/StorageDiskInterface.cfc` — declares put()
  - low `vendor/wheels/mapper/matching.cfc` — declares put()
  - low `vendor/wheels/wheelstest/TestClient.cfc` — declares put()
  - low `vendor/wheels/interfaces/routing/RouteMapperInterface.cfc` — declares put()
  - low `vendor/wheels/storage/drivers/LocalDisk.cfc` — declares put()
  - low `vendor/wheels/storage/drivers/S3Disk.cfc` — declares put()
- `result.post → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — 4 finding(s), 2 candidate(s):
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — declares key()
  - low `vendor/wheels/wheelstest/system/Assertion.cfc` — declares key()
- `variables.wheels.class.adapter → vendor/wheels/databaseAdapters/CockroachDB/CockroachDBModel.cfc` — 4 finding(s), 7 candidate(s):
  - low `vendor/wheels/databaseAdapters/CockroachDB/CockroachDBModel.cfc` — declares $querySetup()
  - low `vendor/wheels/databaseAdapters/H2/H2Model.cfc` — declares $querySetup()
  - low `vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerModel.cfc` — declares $querySetup()
  - low `vendor/wheels/databaseAdapters/MySQL/MySQLModel.cfc` — declares $querySetup()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleModel.cfc` — declares $querySetup()
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLModel.cfc` — declares $querySetup()
  - low `vendor/wheels/databaseAdapters/SQLite/SQLiteModel.cfc` — declares $querySetup()
- `variables.wheels.class.adapter → vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerModel.cfc` — 4 finding(s), 5 candidate(s):
  - low `vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerModel.cfc` — declares $quoteIdentifier(), $getType()
  - low `vendor/wheels/databaseAdapters/MySQL/MySQLModel.cfc` — declares $quoteIdentifier(), $getType()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleModel.cfc` — declares $quoteIdentifier(), $getType()
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLModel.cfc` — declares $quoteIdentifier(), $getType()
  - low `vendor/wheels/databaseAdapters/SQLite/SQLiteModel.cfc` — declares $quoteIdentifier(), $getType()
- `arguments.container → vendor/wheels/tests/_assets/plugins/serviceprovider/FakeContainer.cfc` — 3 finding(s), 4 candidate(s):
  - low `vendor/wheels/tests/_assets/plugins/serviceprovider/FakeContainer.cfc` — declares asSingleton()
  - low `vendor/wheels/tests/_assets/plugins/serviceprovider/TrackingContainer.cfc` — declares asSingleton()
  - low `vendor/wheels/Injector.cfc` — declares asSingleton()
  - low `vendor/wheels/interfaces/di/InjectorInterface.cfc` — declares asSingleton()
- `arguments.target → vendor/wheels/tests/specs/mapperModernSpec.cfc` — 3 finding(s), 67 candidate(s):
  - low `vendor/wheels/tests/specs/mapperModernSpec.cfc` — declares run(), beforeAll(), afterAll()
  - low `vendor/wheels/tests/specs/mapperSpec.cfc` — declares run(), beforeAll(), afterAll()
  - low `vendor/wheels/tests/specs/controller/requestSpec.cfc` — declares run(), beforeAll(), afterAll()
  - low `vendor/wheels/tests/specs/dispatch/RoutePrecedenceSpec.cfc` — declares run(), beforeAll(), afterAll()
  - low `vendor/wheels/tests/specs/dispatch/findMatchingRouteMegaSpec.cfc` — declares run(), beforeAll(), afterAll()
  - low `vendor/wheels/tests/specs/dispatch/findMatchingRouteSpec.cfc` — declares run(), beforeAll(), afterAll()
  - low `vendor/wheels/tests/specs/environment/ipbasedaccessSpec.cfc` — declares run(), beforeAll(), afterAll()
  - low `vendor/wheels/tests/specs/global/addToCacheSpec.cfc` — declares run(), beforeAll(), afterAll()
  - … 59 more
- `arguments.writer → vendor/wheels/tests/_assets/channel/SseWriterFake.cfc` — 3 finding(s), 2 candidate(s):
  - low `vendor/wheels/tests/_assets/channel/SseWriterFake.cfc` — declares flush()
  - low `cli/lucli/services/deploy/lib/Output.cfc` — declares flush()
- `cmds → cli/lucli/services/deploy/cli/DeployAccessoryCli.cfc` — 3 finding(s), 9 candidate(s):
  - low `cli/lucli/services/deploy/cli/DeployAccessoryCli.cfc` — declares start()
  - low `cli/lucli/services/deploy/cli/DeployAppCli.cfc` — declares start()
  - low `cli/lucli/services/deploy/cli/DeployProxyCli.cfc` — declares start()
  - low `cli/lucli/services/deploy/commands/AccessoryCommands.cfc` — declares start()
  - low `cli/lucli/services/deploy/commands/AppCommands.cfc` — declares start()
  - low `cli/lucli/services/deploy/commands/ProxyCommands.cfc` — declares start()
  - low `cli/lucli/Module.cfc` — declares start()
  - low `cli/lucli/services/rustcfml/RustCFMLEngine.cfc` — declares start()
  - … 1 more
- `fixtures.session.scaffold → cli/lucli/services/Scaffold.cfc` — 3 finding(s), 2 candidate(s):
  - medium `cli/lucli/services/Scaffold.cfc` — declares generateAuth(); named like the receiver 'scaffold'
  - low `cli/lucli/Module.cfc` — declares generateAuth()
- `state.adapter → vendor/wheels/databaseAdapters/Abstract.cfc` — 3 finding(s), 4 candidate(s):
  - low `vendor/wheels/databaseAdapters/Abstract.cfc` — declares addColumnOptions()
  - low `vendor/wheels/migrator/ColumnDefinition.cfc` — declares addColumnOptions()
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLMigrator.cfc` — declares addColumnOptions()
  - low `vendor/wheels/interfaces/database/DatabaseMigratorAdapterInterface.cfc` — declares addColumnOptions()
- `this.adapter → vendor/wheels/migrator/Migration.cfc` — 3 finding(s), 5 candidate(s):
  - low `vendor/wheels/migrator/Migration.cfc` — declares dropTable(), createTable()
  - low `vendor/wheels/databaseAdapters/Abstract.cfc` — declares dropTable(), createTable()
  - low `vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerMigrator.cfc` — declares dropTable(), createTable()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleMigrator.cfc` — declares dropTable(), createTable()
  - low `vendor/wheels/interfaces/database/DatabaseMigratorAdapterInterface.cfc` — declares dropTable(), createTable()
- `variables.migration.adapter → vendor/wheels/databaseAdapters/Abstract.cfc` — 3 finding(s), 4 candidate(s):
  - low `vendor/wheels/databaseAdapters/Abstract.cfc` — declares createTable(), quoteTableName()
  - low `vendor/wheels/databaseAdapters/H2/H2Migrator.cfc` — declares createTable(), quoteTableName()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleMigrator.cfc` — declares createTable(), quoteTableName()
  - low `vendor/wheels/interfaces/database/DatabaseMigratorAdapterInterface.cfc` — declares createTable(), quoteTableName()
- `variables.semver → cli/lucli/services/SemVer.cfc` — 3 finding(s), 2 candidate(s):
  - low `cli/lucli/services/SemVer.cfc` — declares satisfiesAll(), compare(); named like the receiver 'semver'
  - low `vendor/wheels/SemVer.cfc` — declares satisfiesAll(), compare(); named like the receiver 'semver'
- `variables.sshpool → cli/lucli/services/deploy/lib/FakeSshPool.cfc` — 3 finding(s), 2 candidate(s):
  - low `cli/lucli/services/deploy/lib/FakeSshPool.cfc` — declares $setSecretValues()
  - low `cli/lucli/services/deploy/lib/SshClient.cfc` — declares $setSecretValues()
- `variables.wheels.class.adapter → vendor/wheels/databaseAdapters/MySQL/MySQLModel.cfc` — 3 finding(s), 3 candidate(s):
  - low `vendor/wheels/databaseAdapters/MySQL/MySQLModel.cfc` — declares $defaultValues(), $querySetup(), $generatedKey()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleModel.cfc` — declares $defaultValues(), $querySetup(), $generatedKey()
  - low `vendor/wheels/databaseAdapters/SQLite/SQLiteModel.cfc` — declares $defaultValues(), $querySetup(), $generatedKey()
- `variables.wheels.class.adapter → vendor/wheels/model/query/QueryBuilder.cfc` — 3 finding(s), 3 candidate(s):
  - low `vendor/wheels/model/query/QueryBuilder.cfc` — declares $quoteValue()
  - low `vendor/wheels/databaseAdapters/Base.cfc` — declares $quoteValue()
  - low `vendor/wheels/interfaces/database/DatabaseModelAdapterInterface.cfc` — declares $quoteValue()
- `adapter → vendor/wheels/migrator/ColumnDefinition.cfc` — 2 finding(s), 4 candidate(s):
  - low `vendor/wheels/migrator/ColumnDefinition.cfc` — declares addColumnOptions()
  - low `vendor/wheels/databaseAdapters/Abstract.cfc` — declares addColumnOptions()
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLMigrator.cfc` — declares addColumnOptions()
  - low `vendor/wheels/interfaces/database/DatabaseMigratorAdapterInterface.cfc` — declares addColumnOptions()
- `analysis → cli/lucli/services/Analysis.cfc` — 2 finding(s), 4 candidate(s):
  - medium `cli/lucli/services/Analysis.cfc` — declares analyze(); named like the receiver 'analysis'
  - low `cli/lucli/Module.cfc` — declares analyze()
  - low `cli/src/models/AnalysisService.cfc` — declares analyze()
  - low `vendor/wheels/wheelstest/system/CodeComplexity.cfc` — declares analyze()
- `arguments.collection → vendor/wheels/Policy.cfc` — 2 finding(s), 5 candidate(s):
  - low `vendor/wheels/Policy.cfc` — declares whereIn()
  - low `vendor/wheels/mapper/matching.cfc` — declares whereIn()
  - low `vendor/wheels/interfaces/routing/RouteMapperInterface.cfc` — declares whereIn()
  - low `vendor/wheels/model/query/QueryBuilder.cfc` — declares whereIn()
  - low `vendor/wheels/tests/_assets/policies/WhereInSpy.cfc` — declares whereIn()
- `arguments.foreignkey → vendor/wheels/migrator/ColumnDefinition.cfc` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/migrator/ColumnDefinition.cfc` — declares toSQL()
  - low `vendor/wheels/migrator/ForeignKeyDefinition.cfc` — declares toSQL()
- `authenticator → vendor/wheels/auth/Authenticator.cfc` — 2 finding(s), 2 candidate(s):
  - medium `vendor/wheels/auth/Authenticator.cfc` — declares hasStrategy(), registerStrategy(); named like the receiver 'authenticator'
  - low `vendor/wheels/auth/AuthenticatorInterface.cfc` — declares hasStrategy(), registerStrategy()
- `core → examples/starter-app/plugins/jsconfirm/JSConfirm.cfc` — 2 finding(s), 5 candidate(s):
  - low `examples/starter-app/plugins/jsconfirm/JSConfirm.cfc` — declares linkTo()
  - low `vendor/wheels/view/links.cfc` — declares linkTo()
  - low `cli/lucli/tests/_helpers/RelatedBlockRenderer.cfc` — declares linkTo()
  - low `vendor/wheels/interfaces/view/ViewLinkInterface.cfc` — declares linkTo()
  - low `vendor/wheels/tests/_assets/controllers/SuperOverride.cfc` — declares linkTo()
- `core → vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner01/Runner01.cfc` — 2 finding(s), 5 candidate(s):
  - low `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner01/Runner01.cfc` — declares URLFor()
  - low `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner02/Runner02.cfc` — declares URLFor()
  - low `vendor/wheels/interfaces/view/ViewLinkInterface.cfc` — declares URLFor()
  - low `vendor/wheels/tests/_assets/plugins/runner/runner01/Runner01.cfc` — declares URLFor()
  - low `vendor/wheels/tests/_assets/plugins/runner/runner02/Runner02.cfc` — declares URLFor()
- `core → vendor/wheels/tests/_assets/plugins/runner/runner01/Runner01.cfc` — 2 finding(s), 5 candidate(s):
  - low `vendor/wheels/tests/_assets/plugins/runner/runner01/Runner01.cfc` — declares URLFor()
  - low `vendor/wheels/tests/_assets/plugins/runner/runner02/Runner02.cfc` — declares URLFor()
  - low `vendor/wheels/interfaces/view/ViewLinkInterface.cfc` — declares URLFor()
  - low `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner01/Runner01.cfc` — declares URLFor()
  - low `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner02/Runner02.cfc` — declares URLFor()
- `ctx.ctrl → vendor/wheels/controller/rendering.cfc` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/controller/rendering.cfc` — declares renderText(), renderNothing()
  - low `vendor/wheels/interfaces/controller/ControllerRenderingInterface.cfc` — declares renderText(), renderNothing()
- `env → cli/lucli/services/deploy/config/Env.cfc` — 2 finding(s), 2 candidate(s):
  - medium `cli/lucli/services/deploy/config/Env.cfc` — declares secret(); named like the receiver 'env'
  - low `vendor/wheels/tests/_assets/controllers/HardenerLifecycle.cfc` — declares secret()
- `local.adapter → vendor/wheels/databaseAdapters/Base.cfc` — 2 finding(s), 8 candidate(s):
  - low `vendor/wheels/databaseAdapters/Base.cfc` — declares $acquireAdvisoryLock(), $releaseAdvisoryLock()
  - low `vendor/wheels/databaseAdapters/CockroachDB/CockroachDBModel.cfc` — declares $acquireAdvisoryLock(), $releaseAdvisoryLock()
  - low `vendor/wheels/databaseAdapters/H2/H2Model.cfc` — declares $acquireAdvisoryLock(), $releaseAdvisoryLock()
  - low `vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerModel.cfc` — declares $acquireAdvisoryLock(), $releaseAdvisoryLock()
  - low `vendor/wheels/databaseAdapters/MySQL/MySQLModel.cfc` — declares $acquireAdvisoryLock(), $releaseAdvisoryLock()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleModel.cfc` — declares $acquireAdvisoryLock(), $releaseAdvisoryLock()
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLModel.cfc` — declares $acquireAdvisoryLock(), $releaseAdvisoryLock()
  - low `vendor/wheels/databaseAdapters/SQLite/SQLiteModel.cfc` — declares $acquireAdvisoryLock(), $releaseAdvisoryLock()
- `local.assoc → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — declares key()
  - low `vendor/wheels/wheelstest/system/Assertion.cfc` — declares key()
- `local.col → vendor/wheels/migrator/ColumnDefinition.cfc` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/migrator/ColumnDefinition.cfc` — declares toSQL()
  - low `vendor/wheels/migrator/ForeignKeyDefinition.cfc` — declares toSQL()
- `local.entry.strategy → vendor/wheels/auth/AuthStrategy.cfc` — 2 finding(s), 12 candidate(s):
  - low `vendor/wheels/auth/AuthStrategy.cfc` — declares authenticate()
  - low `vendor/wheels/auth/Authenticator.cfc` — declares authenticate()
  - low `vendor/wheels/auth/AuthenticatorInterface.cfc` — declares authenticate()
  - low `vendor/wheels/auth/JwtStrategy.cfc` — declares authenticate()
  - low `vendor/wheels/auth/SessionStrategy.cfc` — declares authenticate()
  - low `vendor/wheels/auth/TokenStrategy.cfc` — declares authenticate()
  - low `vendor/wheels/tests/_assets/auth/AlwaysFailStrategy.cfc` — declares authenticate()
  - low `vendor/wheels/tests/_assets/auth/AlwaysPassStrategy.cfc` — declares authenticate()
  - … 4 more
- `migration.adapter → vendor/wheels/databaseAdapters/Abstract.cfc` — 2 finding(s), 7 candidate(s):
  - low `vendor/wheels/databaseAdapters/Abstract.cfc` — declares changeColumnInTable()
  - low `vendor/wheels/databaseAdapters/H2/H2Migrator.cfc` — declares changeColumnInTable()
  - low `vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerMigrator.cfc` — declares changeColumnInTable()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleMigrator.cfc` — declares changeColumnInTable()
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLMigrator.cfc` — declares changeColumnInTable()
  - low `vendor/wheels/databaseAdapters/SQLite/SQLiteMigrator.cfc` — declares changeColumnInTable()
  - low `vendor/wheels/interfaces/database/DatabaseMigratorAdapterInterface.cfc` — declares changeColumnInTable()
- `migration.cfc → vendor/wheels/migrator/Migration.cfc` — 2 finding(s), 47 candidate(s):
  - low `vendor/wheels/migrator/Migration.cfc` — declares up(), down()
  - low `vendor/wheels/migrator/templates/announce.cfc` — declares up(), down()
  - low `vendor/wheels/migrator/templates/blank.cfc` — declares up(), down()
  - low `vendor/wheels/migrator/templates/change-column.cfc` — declares up(), down()
  - low `vendor/wheels/migrator/templates/change-table.cfc` — declares up(), down()
  - low `vendor/wheels/migrator/templates/create-column.cfc` — declares up(), down()
  - low `vendor/wheels/migrator/templates/create-index.cfc` — declares up(), down()
  - low `vendor/wheels/migrator/templates/create-record.cfc` — declares up(), down()
  - … 39 more
- `permission → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — declares key()
  - low `vendor/wheels/wheelstest/system/Assertion.cfc` — declares key()
- `pipeline → vendor/wheels/middleware/Pipeline.cfc` — 2 finding(s), 702 candidate(s):
  - medium `vendor/wheels/middleware/Pipeline.cfc` — declares run(); named like the receiver 'pipeline'
  - low `cli/lucli/services/TestRunner.cfc` — declares run()
  - low `tests/specs/functional/ExampleSpec.cfc` — declares run()
  - low `vendor/wheels/wheelstest/ParallelRunner.cfc` — declares run()
  - low `cli/src/commands/wheels/about.cfc` — declares run()
  - low `cli/src/commands/wheels/benchmark.cfc` — declares run()
  - low `cli/src/commands/wheels/deploy.cfc` — declares run()
  - low `cli/src/commands/wheels/deps.cfc` — declares run()
  - … 694 more
- `result.author → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — declares key()
  - low `vendor/wheels/wheelstest/system/Assertion.cfc` — declares key()
- `results.author → vendor/wheels/migrator/TableDefinition.cfc` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/migrator/TableDefinition.cfc` — declares primaryKey()
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — declares primaryKey()
- `results.shop → vendor/wheels/migrator/TableDefinition.cfc` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/migrator/TableDefinition.cfc` — declares primaryKey()
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — declares primaryKey()
- `s3down → vendor/wheels/interfaces/StorageDiskInterface.cfc` — 2 finding(s), 3 candidate(s):
  - low `vendor/wheels/interfaces/StorageDiskInterface.cfc` — declares put(), exists()
  - low `vendor/wheels/storage/drivers/LocalDisk.cfc` — declares put(), exists()
  - low `vendor/wheels/storage/drivers/S3Disk.cfc` — declares put(), exists()
- `ssh → cli/lucli/tests/specs/deploy/lib/FakeSshPoolSpec.cfc` — 2 finding(s), 702 candidate(s):
  - low `cli/lucli/tests/specs/deploy/lib/FakeSshPoolSpec.cfc` — declares run()
  - low `cli/lucli/tests/specs/deploy/lib/MustacheSpec.cfc` — declares run()
  - low `cli/lucli/tests/specs/deploy/lib/OutputSpec.cfc` — declares run()
  - low `cli/lucli/tests/specs/deploy/lib/SecretResolverSpec.cfc` — declares run()
  - low `cli/lucli/tests/specs/deploy/lib/SshClassloadSpec.cfc` — declares run()
  - low `cli/lucli/tests/specs/deploy/lib/SshClientRedactionSpec.cfc` — declares run()
  - low `cli/lucli/tests/specs/deploy/lib/SshClientSpec.cfc` — declares run()
  - low `cli/lucli/tests/specs/deploy/lib/SshPoolDefaultsSpec.cfc` — declares run()
  - … 694 more
- `svc → cli/lucli/services/Stats.cfc` — 2 finding(s), 2 candidate(s):
  - low `cli/lucli/services/Stats.cfc` — declares getStats()
  - low `vendor/wheels/JobWorker.cfc` — declares getStats()
- `testauthor.profile → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — declares key()
  - low `vendor/wheels/wheelstest/system/Assertion.cfc` — declares key()
- `variables.$class.plugins[] → vendor/wheels/ServiceProviderInterface.cfc` — 2 finding(s), 11 candidate(s):
  - low `vendor/wheels/ServiceProviderInterface.cfc` — declares register()
  - low `vendor/wheels/tests/_assets/packages_hardener_bootresidue/bootresidue/Bootresidue.cfc` — declares register()
  - low `vendor/wheels/tests/_assets/packages_lazy_sp/lazylate/Lazylate.cfc` — declares register()
  - low `vendor/wheels/tests/_assets/packages_lazy_sp/lazysvc/Lazysvc.cfc` — declares register()
  - low `vendor/wheels/tests/_assets/packages_lazy_sp_solo/solounhinted/Solounhinted.cfc` — declares register()
  - low `vendor/wheels/tests/_assets/packages_sp/failboot/Failboot.cfc` — declares register()
  - low `vendor/wheels/tests/_assets/packages_sp/failregister/Failregister.cfc` — declares register()
  - low `vendor/wheels/tests/_assets/packages_sp/goodsp/Goodsp.cfc` — declares register()
  - … 3 more
- `variables.spec → vendor/wheels/wheelstest/system/BaseSpec.cfc` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/wheelstest/system/BaseSpec.cfc` — declares expect()
  - low `cli/lucli/services/deploy/lib/FakeSshPool.cfc` — declares expect()
- `variables.wheels.class.adapter → vendor/wheels/databaseAdapters/H2/H2Model.cfc` — 2 finding(s), 6 candidate(s):
  - low `vendor/wheels/databaseAdapters/H2/H2Model.cfc` — declares $upsertSQL(), $querySetup()
  - low `vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerModel.cfc` — declares $upsertSQL(), $querySetup()
  - low `vendor/wheels/databaseAdapters/MySQL/MySQLModel.cfc` — declares $upsertSQL(), $querySetup()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleModel.cfc` — declares $upsertSQL(), $querySetup()
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLModel.cfc` — declares $upsertSQL(), $querySetup()
  - low `vendor/wheels/databaseAdapters/SQLite/SQLiteModel.cfc` — declares $upsertSQL(), $querySetup()
- `a → cli/lucli/services/deploy/config/Accessory.cfc` — 1 finding(s), 9 candidate(s):
  - low `cli/lucli/services/deploy/config/Accessory.cfc` — declares name()
  - low `cli/lucli/services/deploy/config/Role.cfc` — declares name()
  - low `cli/lucli/services/deploy/secrets/AwsSecretsAdapter.cfc` — declares name()
  - low `cli/lucli/services/deploy/secrets/BaseAdapter.cfc` — declares name()
  - low `cli/lucli/services/deploy/secrets/BitwardenAdapter.cfc` — declares name()
  - low `cli/lucli/services/deploy/secrets/DopplerAdapter.cfc` — declares name()
  - low `cli/lucli/services/deploy/secrets/LastPassAdapter.cfc` — declares name()
  - low `cli/lucli/services/deploy/secrets/OnePasswordAdapter.cfc` — declares name()
  - … 1 more
- `acc → cli/lucli/services/deploy/config/Env.cfc` — 1 finding(s), 2 candidate(s):
  - medium `cli/lucli/services/deploy/config/Env.cfc` — declares secret(); named like the receiver 'env'
  - low `vendor/wheels/tests/_assets/controllers/HardenerLifecycle.cfc` — declares secret()
- `adapter → cli/lucli/services/deploy/cli/DeploySecretsCli.cfc` — 1 finding(s), 8 candidate(s):
  - low `cli/lucli/services/deploy/cli/DeploySecretsCli.cfc` — declares fetch()
  - low `cli/lucli/services/deploy/secrets/AwsSecretsAdapter.cfc` — declares fetch()
  - low `cli/lucli/services/deploy/secrets/BaseAdapter.cfc` — declares fetch()
  - low `cli/lucli/services/deploy/secrets/BitwardenAdapter.cfc` — declares fetch()
  - low `cli/lucli/services/deploy/secrets/DopplerAdapter.cfc` — declares fetch()
  - low `cli/lucli/services/deploy/secrets/LastPassAdapter.cfc` — declares fetch()
  - low `cli/lucli/services/deploy/secrets/OnePasswordAdapter.cfc` — declares fetch()
  - low `cli/lucli/services/deploy/secrets/SecretAdapterInterface.cfc` — declares fetch()
- `adapter → vendor/wheels/migrator/ForeignKeyDefinition.cfc` — 1 finding(s), 7 candidate(s):
  - low `vendor/wheels/migrator/ForeignKeyDefinition.cfc` — declares addForeignKeyOptions()
  - low `vendor/wheels/databaseAdapters/Abstract.cfc` — declares addForeignKeyOptions()
  - low `vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerMigrator.cfc` — declares addForeignKeyOptions()
  - low `vendor/wheels/databaseAdapters/MySQL/MySQLMigrator.cfc` — declares addForeignKeyOptions()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleMigrator.cfc` — declares addForeignKeyOptions()
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLMigrator.cfc` — declares addForeignKeyOptions()
  - low `vendor/wheels/databaseAdapters/SQLite/SQLiteMigrator.cfc` — declares addForeignKeyOptions()
- `apimap → vendor/wheels/BuildInfo.cfc` — 1 finding(s), 6 candidate(s):
  - low `vendor/wheels/BuildInfo.cfc` — declares version()
  - low `vendor/wheels/mapper/scoping.cfc` — declares version()
  - low `vendor/wheels/interfaces/routing/RouteMapperInterface.cfc` — declares version()
  - low `cli/lucli/Module.cfc` — declares version()
  - low `cli/lucli/tests/_modules/BaseModule.cfc` — declares version()
  - low `cli/lucli/services/deploy/cli/DeployMainCli.cfc` — declares version()
- `application.$wheels.buildinfo → vendor/wheels/BuildInfo.cfc` — 1 finding(s), 6 candidate(s):
  - medium `vendor/wheels/BuildInfo.cfc` — declares version(); named like the receiver 'buildInfo'
  - low `vendor/wheels/mapper/scoping.cfc` — declares version()
  - low `vendor/wheels/interfaces/routing/RouteMapperInterface.cfc` — declares version()
  - low `cli/lucli/Module.cfc` — declares version()
  - low `cli/lucli/tests/_modules/BaseModule.cfc` — declares version()
  - low `cli/lucli/services/deploy/cli/DeployMainCli.cfc` — declares version()
- `application.$wheels.engineadapter → vendor/wheels/engineAdapters/Base.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/engineAdapters/Base.cfc` — declares prepareDIComplete()
  - low `vendor/wheels/engineAdapters/BoxLang/BoxLangAdapter.cfc` — declares prepareDIComplete()
- `application.wheels.buildinfo → vendor/wheels/BuildInfo.cfc` — 1 finding(s), 6 candidate(s):
  - medium `vendor/wheels/BuildInfo.cfc` — declares version(); named like the receiver 'buildInfo'
  - low `vendor/wheels/mapper/scoping.cfc` — declares version()
  - low `vendor/wheels/interfaces/routing/RouteMapperInterface.cfc` — declares version()
  - low `cli/lucli/Module.cfc` — declares version()
  - low `cli/lucli/tests/_modules/BaseModule.cfc` — declares version()
  - low `cli/lucli/services/deploy/cli/DeployMainCli.cfc` — declares version()
- `application.wheels.public → vendor/wheels/Public.cfc` — 1 finding(s), 2 candidate(s):
  - medium `vendor/wheels/Public.cfc` — declares $loadRegistryPackages(); named like the receiver 'public'
  - low `vendor/wheels/tests/_assets/packages/FakePublic.cfc` — declares $loadRegistryPackages()
- `application.wheels.public → vendor/wheels/tests/_assets/dispatch/InvokeMethodFixture.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/tests/_assets/dispatch/InvokeMethodFixture.cfc` — declares getState()
  - low `vendor/wheels/wheelstest/BrowserLauncher.cfc` — declares getState()
- `application[].pluginobj → vendor/wheels/PackageLoader.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/PackageLoader.cfc` — declares getMethodProviders()
  - low `vendor/wheels/Plugins.cfc` — declares getMethodProviders()
- `arguments.adapter → vendor/wheels/databaseAdapters/CockroachDB/CockroachDBMigrator.cfc` — 1 finding(s), 7 candidate(s):
  - low `vendor/wheels/databaseAdapters/CockroachDB/CockroachDBMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/H2/H2Migrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/MySQL/MySQLMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/SQLite/SQLiteMigrator.cfc` — declares adapterName()
- `arguments.applicationscope.$wheelsbrowserlauncher → cli/lucli/services/deploy/cli/DeployLockCli.cfc` — 1 finding(s), 3 candidate(s):
  - low `cli/lucli/services/deploy/cli/DeployLockCli.cfc` — declares release()
  - low `cli/lucli/services/deploy/commands/LockCommands.cfc` — declares release()
  - low `vendor/wheels/wheelstest/BrowserLauncher.cfc` — declares release()
- `arguments.applicationscope.$wheelsbrowserlauncher → vendor/wheels/wheelstest/BrowserLauncher.cfc` — 1 finding(s), 3 candidate(s):
  - low `vendor/wheels/wheelstest/BrowserLauncher.cfc` — declares release()
  - low `cli/lucli/services/deploy/cli/DeployLockCli.cfc` — declares release()
  - low `cli/lucli/services/deploy/commands/LockCommands.cfc` — declares release()
- `arguments.javasystem → cli/lucli/Module.cfc` — 1 finding(s), 2 candidate(s):
  - low `cli/lucli/Module.cfc` — declares console()
  - low `vendor/wheels/wheelstest/system/BaseSpec.cfc` — declares console()
- `arguments.plugin → vendor/wheels/tests/_assets/plugins/hardener_mutateapp/MutateAppPlugin/MutateAppPlugin.cfc` — 1 finding(s), 7 candidate(s):
  - low `vendor/wheels/tests/_assets/plugins/hardener_mutateapp/MutateAppPlugin/MutateAppPlugin.cfc` — declares onPluginLoad()
  - low `vendor/wheels/tests/_assets/plugins/lifecycle/TestLifecyclePluginA/TestLifecyclePluginA.cfc` — declares onPluginLoad()
  - low `vendor/wheels/tests/_assets/plugins/lifecycle/TestLifecyclePluginB/TestLifecyclePluginB.cfc` — declares onPluginLoad()
  - low `vendor/wheels/tests/_assets/plugins/lifecyclefailing/TestLifecycleFailingA/TestLifecycleFailingA.cfc` — declares onPluginLoad()
  - low `vendor/wheels/tests/_assets/plugins/lifecyclefailing/TestLifecycleWorkingB/TestLifecycleWorkingB.cfc` — declares onPluginLoad()
  - low `vendor/wheels/tests/_assets/plugins/middleware/TestMiddlewarePluginA/TestMiddlewarePluginA.cfc` — declares onPluginLoad()
  - low `vendor/wheels/tests/_assets/plugins/middleware/TestMiddlewarePluginB/TestMiddlewarePluginB.cfc` — declares onPluginLoad()
- `arguments.targetobject → vendor/wheels/wheelstest/system/mockutils/MockGenerator.cfc` — 1 finding(s), 3 candidate(s):
  - low `vendor/wheels/wheelstest/system/mockutils/MockGenerator.cfc` — declares $include()
  - low `vendor/wheels/Global.cfc` — declares $include()
  - low `vendor/wheels/tests/_assets/events/CorsArbitrationEventDouble.cfc` — declares $include()
- `attributes.box.instance → vendor/wheels/Job.cfc` — 1 finding(s), 12 candidate(s):
  - low `vendor/wheels/Job.cfc` — declares perform()
  - low `vendor/wheels/tests/_assets/jobs/ConfigBackoffJob.cfc` — declares perform()
  - low `vendor/wheels/tests/_assets/jobs/FailingBackoffJob.cfc` — declares perform()
  - low `vendor/wheels/tests/_assets/jobs/LinearBackoffJob.cfc` — declares perform()
  - low `vendor/wheels/tests/_assets/jobs/ProbeJob.cfc` — declares perform()
  - low `vendor/wheels/tests/_assets/jobs/SlowTimeoutJob.cfc` — declares perform()
  - low `vendor/wheels/tests/_assets/jobs/StealDuringPerformJob.cfc` — declares perform()
  - low `vendor/wheels/tests/_assets/jobs/StealToFailedJob.cfc` — declares perform()
  - … 4 more
- `cmds → cli/lucli/services/TestRunner.cfc` — 1 finding(s), 702 candidate(s):
  - low `cli/lucli/services/TestRunner.cfc` — declares run()
  - low `cli/lucli/services/deploy/commands/AccessoryCommands.cfc` — declares run()
  - low `cli/lucli/services/deploy/commands/AppCommands.cfc` — declares run()
  - low `cli/lucli/services/deploy/lib/SshClient.cfc` — declares run()
  - low `cli/lucli/tests/specs/commands/CliHardenerS1S10Spec.cfc` — declares run()
  - low `cli/lucli/tests/specs/commands/CommandArgParsingSpec.cfc` — declares run()
  - low `cli/lucli/tests/specs/commands/ConsoleCommandSpec.cfc` — declares run()
  - low `cli/lucli/tests/specs/commands/CreateCommandSpec.cfc` — declares run()
  - … 694 more
- `cmds → cli/lucli/services/deploy/cli/DeployLockCli.cfc` — 1 finding(s), 4 candidate(s):
  - low `cli/lucli/services/deploy/cli/DeployLockCli.cfc` — declares status()
  - low `cli/lucli/services/deploy/commands/AppCommands.cfc` — declares status()
  - low `cli/lucli/services/deploy/commands/LockCommands.cfc` — declares status()
  - low `cli/lucli/services/rustcfml/RustCFMLEngine.cfc` — declares status()
- `containerreceived → vendor/wheels/Injector.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/Injector.cfc` — declares getInstance()
  - low `vendor/wheels/interfaces/di/InjectorInterface.cfc` — declares getInstance()
- `core → examples/starter-app/plugins/FlashMessagesBootstrap/FlashMessagesBootstrap.cfc` — 1 finding(s), 2 candidate(s):
  - low `examples/starter-app/plugins/FlashMessagesBootstrap/FlashMessagesBootstrap.cfc` — declares flashMessages()
  - low `vendor/wheels/view/miscellaneous.cfc` — declares flashMessages()
- `core → vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner02/Runner02.cfc` — 1 finding(s), 5 candidate(s):
  - low `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner02/Runner02.cfc` — declares URLFor()
  - low `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner01/Runner01.cfc` — declares URLFor()
  - low `vendor/wheels/interfaces/view/ViewLinkInterface.cfc` — declares URLFor()
  - low `vendor/wheels/tests/_assets/plugins/runner/runner01/Runner01.cfc` — declares URLFor()
  - low `vendor/wheels/tests/_assets/plugins/runner/runner02/Runner02.cfc` — declares URLFor()
- `core → vendor/wheels/tests/_assets/plugins/runner/runner02/Runner02.cfc` — 1 finding(s), 5 candidate(s):
  - low `vendor/wheels/tests/_assets/plugins/runner/runner02/Runner02.cfc` — declares URLFor()
  - low `vendor/wheels/tests/_assets/plugins/runner/runner01/Runner01.cfc` — declares URLFor()
  - low `vendor/wheels/interfaces/view/ViewLinkInterface.cfc` — declares URLFor()
  - low `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner01/Runner01.cfc` — declares URLFor()
  - low `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner02/Runner02.cfc` — declares URLFor()
- `ctx → vendor/wheels/Injector.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/Injector.cfc` — declares getInstance()
  - low `vendor/wheels/interfaces/di/InjectorInterface.cfc` — declares getInstance()
- `ctx.mw → vendor/wheels/middleware/AuthMiddleware.cfc` — 1 finding(s), 16 candidate(s):
  - low `vendor/wheels/middleware/AuthMiddleware.cfc` — declares handle()
  - low `vendor/wheels/middleware/BrowserTestFixtureGuard.cfc` — declares handle()
  - low `vendor/wheels/middleware/Cors.cfc` — declares handle()
  - low `vendor/wheels/middleware/MiddlewareInterface.cfc` — declares handle()
  - low `vendor/wheels/middleware/RateLimiter.cfc` — declares handle()
  - low `vendor/wheels/middleware/RequestId.cfc` — declares handle()
  - low `vendor/wheels/middleware/SecurityHeaders.cfc` — declares handle()
  - low `vendor/wheels/middleware/TenantResolver.cfc` — declares handle()
  - … 8 more
- `idata.type → vendor/wheels/wheelstest/system/reports/ANTJUnitReporter.cfc` — 1 finding(s), 4 candidate(s):
  - low `vendor/wheels/wheelstest/system/reports/ANTJUnitReporter.cfc` — declares runReport()
  - low `vendor/wheels/wheelstest/system/reports/IReporter.cfc` — declares runReport()
  - low `vendor/wheels/wheelstest/system/reports/JSONReporter.cfc` — declares runReport()
  - low `vendor/wheels/wheelstest/system/reports/TextReporter.cfc` — declares runReport()
- `idx.i → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — declares key()
  - low `vendor/wheels/wheelstest/system/Assertion.cfc` — declares key()
- `innermap → vendor/wheels/wheelstest/TestClient.cfc` — 1 finding(s), 6 candidate(s):
  - low `vendor/wheels/wheelstest/TestClient.cfc` — declares put()
  - low `vendor/wheels/interfaces/StorageDiskInterface.cfc` — declares put()
  - low `vendor/wheels/mapper/matching.cfc` — declares put()
  - low `vendor/wheels/interfaces/routing/RouteMapperInterface.cfc` — declares put()
  - low `vendor/wheels/storage/drivers/LocalDisk.cfc` — declares put()
  - low `vendor/wheels/storage/drivers/S3Disk.cfc` — declares put()
- `launcher → vendor/wheels/tests/_assets/dispatch/InvokeMethodFixture.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/tests/_assets/dispatch/InvokeMethodFixture.cfc` — declares getState()
  - low `vendor/wheels/wheelstest/BrowserLauncher.cfc` — declares getState()
- `local.candidate.strategy → vendor/wheels/auth/AuthStrategy.cfc` — 1 finding(s), 8 candidate(s):
  - low `vendor/wheels/auth/AuthStrategy.cfc` — declares supports()
  - low `vendor/wheels/auth/JwtStrategy.cfc` — declares supports()
  - low `vendor/wheels/auth/SessionStrategy.cfc` — declares supports()
  - low `vendor/wheels/auth/TokenStrategy.cfc` — declares supports()
  - low `vendor/wheels/tests/_assets/auth/AlwaysFailStrategy.cfc` — declares supports()
  - low `vendor/wheels/tests/_assets/auth/AlwaysPassStrategy.cfc` — declares supports()
  - low `vendor/wheels/tests/_assets/auth/HeaderTokenStrategy.cfc` — declares supports()
  - low `vendor/wheels/tests/_assets/auth/UnsupportedStrategy.cfc` — declares supports()
- `local.classdata.adapter → vendor/wheels/model/query/QueryBuilder.cfc` — 1 finding(s), 3 candidate(s):
  - low `vendor/wheels/model/query/QueryBuilder.cfc` — declares $quoteValue()
  - low `vendor/wheels/databaseAdapters/Base.cfc` — declares $quoteValue()
  - low `vendor/wheels/interfaces/database/DatabaseModelAdapterInterface.cfc` — declares $quoteValue()
- `local.defaultentry.strategy → vendor/wheels/auth/AuthStrategy.cfc` — 1 finding(s), 8 candidate(s):
  - low `vendor/wheels/auth/AuthStrategy.cfc` — declares supports()
  - low `vendor/wheels/auth/JwtStrategy.cfc` — declares supports()
  - low `vendor/wheels/auth/SessionStrategy.cfc` — declares supports()
  - low `vendor/wheels/auth/TokenStrategy.cfc` — declares supports()
  - low `vendor/wheels/tests/_assets/auth/AlwaysFailStrategy.cfc` — declares supports()
  - low `vendor/wheels/tests/_assets/auth/AlwaysPassStrategy.cfc` — declares supports()
  - low `vendor/wheels/tests/_assets/auth/HeaderTokenStrategy.cfc` — declares supports()
  - low `vendor/wheels/tests/_assets/auth/UnsupportedStrategy.cfc` — declares supports()
- `local.model → vendor/wheels/migrator/TableDefinition.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/migrator/TableDefinition.cfc` — declares primaryKey()
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — declares primaryKey()
- `local.modelclass → vendor/wheels/interfaces/model/ModelFinderInterface.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/interfaces/model/ModelFinderInterface.cfc` — declares findByKey()
  - low `vendor/wheels/model/query/ScopeChain.cfc` — declares findByKey()
- `local.modelobj → vendor/wheels/interfaces/model/ModelFinderInterface.cfc` — 1 finding(s), 3 candidate(s):
  - low `vendor/wheels/interfaces/model/ModelFinderInterface.cfc` — declares findOne()
  - low `vendor/wheels/model/query/QueryBuilder.cfc` — declares findOne()
  - low `vendor/wheels/model/query/ScopeChain.cfc` — declares findOne()
- `local.plugin → vendor/wheels/tests/_assets/plugins/hardener_mutateapp/MutateAppPlugin/MutateAppPlugin.cfc` — 1 finding(s), 5 candidate(s):
  - low `vendor/wheels/tests/_assets/plugins/hardener_mutateapp/MutateAppPlugin/MutateAppPlugin.cfc` — declares onPluginActivate()
  - low `vendor/wheels/tests/_assets/plugins/lifecycle/TestLifecyclePluginA/TestLifecyclePluginA.cfc` — declares onPluginActivate()
  - low `vendor/wheels/tests/_assets/plugins/lifecycle/TestLifecyclePluginB/TestLifecyclePluginB.cfc` — declares onPluginActivate()
  - low `vendor/wheels/tests/_assets/plugins/lifecyclefailing/TestLifecycleFailingA/TestLifecycleFailingA.cfc` — declares onPluginActivate()
  - low `vendor/wheels/tests/_assets/plugins/lifecyclefailing/TestLifecycleWorkingB/TestLifecycleWorkingB.cfc` — declares onPluginActivate()
- `local.policy → vendor/wheels/Policy.cfc` — 1 finding(s), 5 candidate(s):
  - medium `vendor/wheels/Policy.cfc` — declares scope(); named like the receiver 'policy'
  - low `vendor/wheels/mapper/scoping.cfc` — declares scope()
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — declares scope()
  - low `vendor/wheels/interfaces/routing/RouteMapperInterface.cfc` — declares scope()
  - low `vendor/wheels/tests/_assets/policies/PostPolicy.cfc` — declares scope()
- `local.reg → vendor/wheels/services/packages/Registry.cfc` — 1 finding(s), 3 candidate(s):
  - low `vendor/wheels/services/packages/Registry.cfc` — declares listAll()
  - low `vendor/wheels/tests/_assets/packages/FakeRegistry.cfc` — declares listAll()
  - low `cli/lucli/services/packages/Registry.cfc` — declares listAll()
- `local.strategy → vendor/wheels/Policy.cfc` — 1 finding(s), 3 candidate(s):
  - low `vendor/wheels/Policy.cfc` — declares currentUser()
  - low `vendor/wheels/auth/SessionStrategy.cfc` — declares currentUser()
  - low `vendor/wheels/tests/_assets/policies/ThrowingCurrentUserStrategy.cfc` — declares currentUser()
- `local.value → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — declares key()
  - low `vendor/wheels/wheelstest/system/Assertion.cfc` — declares key()
- `map → vendor/wheels/mapper/scoping.cfc` — 1 finding(s), 3 candidate(s):
  - low `vendor/wheels/mapper/scoping.cfc` — declares group()
  - low `vendor/wheels/interfaces/routing/RouteMapperInterface.cfc` — declares group()
  - low `vendor/wheels/model/query/QueryBuilder.cfc` — declares group()
- `omockgenerator → vendor/wheels/wheelstest/system/mockutils/MockGenerator.cfc` — 1 finding(s), 2 candidate(s):
  - medium `vendor/wheels/wheelstest/system/mockutils/MockGenerator.cfc` — declares generate(); named like the receiver 'oMockGenerator'
  - low `cli/lucli/Module.cfc` — declares generate()
- `os → cli/lucli/services/deploy/lib/Output.cfc` — 1 finding(s), 2 candidate(s):
  - low `cli/lucli/services/deploy/lib/Output.cfc` — declares flush()
  - low `vendor/wheels/tests/_assets/channel/SseWriterFake.cfc` — declares flush()
- `outstream → cli/lucli/services/deploy/lib/Output.cfc` — 1 finding(s), 2 candidate(s):
  - low `cli/lucli/services/deploy/lib/Output.cfc` — declares flush()
  - low `vendor/wheels/tests/_assets/channel/SseWriterFake.cfc` — declares flush()
- `setting → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — declares key()
  - low `vendor/wheels/wheelstest/system/Assertion.cfc` — declares key()
- `snippet → cli/lucli/Module.cfc` — 1 finding(s), 2 candidate(s):
  - low `cli/lucli/Module.cfc` — declares generate()
  - low `vendor/wheels/wheelstest/system/mockutils/MockGenerator.cfc` — declares generate()
- `svc → cli/lucli/Module.cfc` — 1 finding(s), 2 candidate(s):
  - low `cli/lucli/Module.cfc` — declares generateAdmin()
  - low `cli/lucli/services/Admin.cfc` — declares generateAdmin()
- `this.containerreceived → vendor/wheels/tests/_assets/plugins/serviceprovider/TrackingContainer.cfc` — 1 finding(s), 3 candidate(s):
  - low `vendor/wheels/tests/_assets/plugins/serviceprovider/TrackingContainer.cfc` — declares containsInstance()
  - low `vendor/wheels/Injector.cfc` — declares containsInstance()
  - low `vendor/wheels/interfaces/di/InjectorInterface.cfc` — declares containsInstance()
- `v1 → vendor/wheels/mapper/resources.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/mapper/resources.cfc` — declares resources()
  - low `vendor/wheels/interfaces/routing/RouteMapperInterface.cfc` — declares resources()
- `variables.fkadapter → vendor/wheels/databaseAdapters/Abstract.cfc` — 1 finding(s), 5 candidate(s):
  - low `vendor/wheels/databaseAdapters/Abstract.cfc` — declares createTable()
  - low `vendor/wheels/migrator/Migration.cfc` — declares createTable()
  - low `vendor/wheels/databaseAdapters/H2/H2Migrator.cfc` — declares createTable()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleMigrator.cfc` — declares createTable()
  - low `vendor/wheels/interfaces/database/DatabaseMigratorAdapterInterface.cfc` — declares createTable()
- `variables.http → cli/lucli/services/packages/HttpClient.cfc` — 1 finding(s), 3 candidate(s):
  - low `cli/lucli/services/packages/HttpClient.cfc` — declares download()
  - low `cli/lucli/services/deploy/lib/SshClient.cfc` — declares download()
  - low `cli/lucli/tests/specs/packages/_stubs/FakeHttpClient.cfc` — declares download()
- `variables.migration.adapter → vendor/wheels/databaseAdapters/CockroachDB/CockroachDBMigrator.cfc` — 1 finding(s), 7 candidate(s):
  - low `vendor/wheels/databaseAdapters/CockroachDB/CockroachDBMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/H2/H2Migrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/MySQL/MySQLMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/Oracle/OracleMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLMigrator.cfc` — declares adapterName()
  - low `vendor/wheels/databaseAdapters/SQLite/SQLiteMigrator.cfc` — declares adapterName()
- `variables.page → vendor/wheels/interfaces/model/ModelFinderInterface.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/interfaces/model/ModelFinderInterface.cfc` — declares reload()
  - low `cli/lucli/Module.cfc` — declares reload()
- `variables.simpleservice → vendor/wheels/tests/_assets/di/SimpleService.cfc` — 1 finding(s), 2 candidate(s):
  - medium `vendor/wheels/tests/_assets/di/SimpleService.cfc` — declares greet(); named like the receiver 'simpleService'
  - low `vendor/wheels/tests/_assets/plugins/serviceprovider/TestServiceProvider/PluginGreetingService.cfc` — declares greet()

</details>

## Return types — a call chained on a method that declares no component

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 41 | `method '$engineAdapter' has no component return type → vendor/wheels/engineAdapters/Base.cfc` | 2 | low | declares prepareDIComplete() (2 candidates) | `vendor/wheels/Controller.cfc:433` method '$engineAdapter' has no component return type (chain to 'prepareDIComplete') |
| 13 | `method 'error' in DetailOutputService@wheels-cli has no component return type → cli/src/models/DetailOutputService.cfc` | 1 | medium | declares line() | `cli/src/commands/wheels/destroy.cfc:28` method 'error' in DetailOutputService@wheels-cli has no component return type (chain to 'line') |
| 12 | `method '$engineAdapter' in wheels.Global has no component return type → vendor/wheels/engineAdapters/Base.cfc` | 4 | low | declares isLucee() (3 candidates) | `vendor/wheels/tests/specs/dispatch/setCorsHeadersSpec.cfc:16` method '$engineAdapter' in wheels.Global has no component return type (chain to 'isLucee') |
| 10 | `method '$locator' has no component return type` | 0 | none |  | `vendor/wheels/wheelstest/BrowserClient.cfc:149` method '$locator' has no component return type (chain to 'pressSequentially') |
| 9 | `method '$jobBridge' has no component return type → vendor/wheels/Job.cfc` | 1 | high | declares $instantiateJobClass(), $restoreTenantContext(), $takeJobTimeout(), $runPerformWithTimeout(), $clearTenantContext() | `vendor/wheels/JobWorker.cfc:528` method '$jobBridge' has no component return type (chain to '$instantiateJobClass') |
| 7 | `method '$mapper' has no component return type → vendor/wheels/mapper/mapping.cfc` | 1 | medium | declares $draw() | `vendor/wheels/tests/specs/mapper/MapperHardenerShouldSpec.cfc:66` method '$mapper' has no component return type (chain to '$draw') |
| 7 | `method 'accessory' in Config has no component return type → cli/lucli/services/deploy/config/Accessory.cfc` | 2 | high | declares port(), volumes(), containerName(), labelService(); named like the receiver 'accessory' | `cli/lucli/tests/specs/deploy/config/AccessorySpec.cfc:49` method 'accessory' in Config has no component return type (chain to 'port') |
| 6 | `method '$locator' has no component return type → vendor/wheels/wheelstest/BrowserClient.cfc` | 1 | medium | declares click() | `vendor/wheels/wheelstest/BrowserClient.cfc:109` method '$locator' has no component return type (chain to 'click') |
| 6 | `method 'getService' has no component return type → cli/lucli/services/Helpers.cfc` | 2 | low | declares generateMigrationTimestamp() (2 candidates) | `cli/lucli/Module.cfc:4904` method 'getService' has no component return type (chain to 'generateMigrationTimestamp') |
| 4 | `method 'getInstance' in wheels.Injector has no component return type → vendor/wheels/Dispatch.cfc` | 8 | low | declares $init() (8 candidates) | `cli/lucli/templates/app/public/Application.cfc:167` method 'getInstance' in wheels.Injector has no component return type (chain to '$init') |
| 4 | `method 'getInstance' in wheels.Injector has no component return type → vendor/wheels/tests/_assets/di/SimpleService.cfc` | 1 | medium | declares getMarker() | `vendor/wheels/tests/specs/injector/InjectorHardenerSpec.cfc:177` method 'getInstance' in wheels.Injector has no component return type (chain to 'getMarker') |
| 4 | `method 'getUtility' has no component return type → vendor/wheels/wheelstest/system/util/Util.cfc` | 2 | medium | declares getAnnotatedMethods() | `vendor/wheels/wheelstest/system/BaseSpec.cfc:1171` method 'getUtility' has no component return type (chain to 'getAnnotatedMethods') |
| 3 | `method '$engineCapabilities' in wheels.wheelstest.BrowserLauncher has no component return type → vendor/wheels/wheelstest/EngineCapabilities.cfc` | 1 | medium | declares hasJvmClassLoading() | `vendor/wheels/tests/specs/wheelstest/BrowserIntegrationSpec.cfc:10` method '$engineCapabilities' in wheels.wheelstest.BrowserLauncher has no component return type (chain to 'hasJvmClassLoading') |
| 3 | `method 'getJavaSystem' has no component return type → vendor/wheels/wheelstest/system/BaseSpec.cfc` | 2 | high | declares getProperty(), getEnv() | `vendor/wheels/wheelstest/system/util/Env.cfc:15` method 'getJavaSystem' has no component return type (chain to 'getProperty') |
| 2 | `method 'getService' has no component return type → cli/lucli/services/PortProbe.cfc` | 1 | medium | declares portInUse() | `cli/lucli/Module.cfc:1608` method 'getService' has no component return type (chain to 'portInUse') |
| 2 | `method 'model' has no component return type` | 1 | none |  | `vendor/wheels/model/onmissingmethod.cfm:358` method 'model' has no component return type (chain to '$expandedAssociations') |
| 1 | `method '$classData' in author has no component return type → vendor/wheels/databaseAdapters/Base.cfc` | 6 | low | declares $supportsAdvisoryLocks() (6 candidates) | `vendor/wheels/tests/specs/model/lockingSpec.cfc:20` method '$classData' in author has no component return type (chain to '$supportsAdvisoryLocks') |
| 1 | `method '$classData' in post has no component return type → vendor/wheels/databaseAdapters/Base.cfc` | 2 | low | declares $isSharedModel() (2 candidates) | `vendor/wheels/tests/specs/model/MultiTenantSpec.cfc:147` method '$classData' in post has no component return type (chain to '$isSharedModel') |
| 1 | `method '$engineCapabilities' has no component return type → vendor/wheels/wheelstest/EngineCapabilities.cfc` | 1 | medium | declares hasJvmClassLoading() | `vendor/wheels/wheelstest/BrowserLauncher.cfc:207` method '$engineCapabilities' has no component return type (chain to 'hasJvmClassLoading') |
| 1 | `method '$getObject' has no component return type → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` | 4 | low | declares key() (2 candidates) | `vendor/wheels/view/miscellaneous.cfc:743` method '$getObject' has no component return type (chain to 'key') |
| 1 | `method '$modelSuperPrototype' has no component return type → vendor/wheels/Model.cfc` | 1 | medium | declares $superOriginal() | `vendor/wheels/Model.cfc:69` method '$modelSuperPrototype' has no component return type (chain to '$superOriginal') |
| 1 | `method 'getCBMockData' has no component return type` | 0 | none |  | `vendor/wheels/wheelstest/system/BaseSpec.cfc:1617` method 'getCBMockData' has no component return type (chain to 'mock') |
| 1 | `method 'getClassLoader' has no component return type → cli/lucli/services/deploy/lib/JarLoader.cfc` | 1 | medium | declares loadClass() | `cli/lucli/services/deploy/lib/JarLoader.cfc:86` method 'getClassLoader' has no component return type (chain to 'loadClass') |
| 1 | `method 'getClassLoader' in wheels.wheelstest.BrowserLauncher has no component return type → cli/lucli/services/deploy/lib/JarLoader.cfc` | 1 | medium | declares loadClass() | `vendor/wheels/tests/specs/wheelstest/BrowserLauncherSpec.cfc:134` method 'getClassLoader' in wheels.wheelstest.BrowserLauncher has no component return type (chain to 'loadClass') |
| 1 | `method 'getJavaSystem' has no component return type → vendor/wheels/wheelstest/system/util/Env.cfc` | 7 | low | declares getEnv() (4 candidates) | `vendor/wheels/wheelstest/system/util/Env.cfc:68` method 'getJavaSystem' has no component return type (chain to 'getEnv') |
| 1 | `method 'getLauncher' in wheels.wheelstest.BrowserClient has no component return type → vendor/wheels/tests/_assets/dispatch/InvokeMethodFixture.cfc` | 2 | low | declares getState() (2 candidates) | `vendor/wheels/tests/specs/wheelstest/BrowserIntegrationSpec.cfc:99` method 'getLauncher' in wheels.wheelstest.BrowserClient has no component return type (chain to 'getState') |
| 1 | `method 'getService' has no component return type → cli/lucli/services/ServerRegistry.cfc` | 1 | medium | declares ownServerPort() | `cli/lucli/Module.cfc:9732` method 'getService' has no component return type (chain to 'ownServerPort') |
| 1 | `method 'getUtility' has no component return type → vendor/wheels/wheelstest/system/util/MixerUtil.cfc` | 10 | medium | declares start(); named like the receiver 'MixerUtil' (9 candidates) | `vendor/wheels/wheelstest/system/BaseSpec.cfc:1576` method 'getUtility' has no component return type (chain to 'start') |
| 1 | `method 'model' has no component return type → vendor/wheels/Model.cfc` | 1 | high | declares $classData(); named like the receiver 'model' | `vendor/wheels/Seeder.cfc:456` method 'model' has no component return type (chain to '$classData') |
| 1 | `method 'model' in wheels.Global has no component return type → vendor/wheels/Model.cfc` | 1 | high | declares $classData(); named like the receiver 'model' | `vendor/wheels/tests/specs/model/ExpandedAssociationsJoinMemoSpec.cfc:87` method 'model' in wheels.Global has no component return type (chain to '$classData') |
| 1 | `method 'policyScope' in Authorization has no component return type → vendor/wheels/Policy.cfc` | 2 | low | declares where() (2 candidates) | `vendor/wheels/tests/specs/Authorization/AuthorizationSpec.cfc:282` method 'policyScope' in Authorization has no component return type (chain to 'where') |

<details><summary>Groups with several candidates</summary>

- `method '$engineAdapter' has no component return type → vendor/wheels/engineAdapters/Base.cfc` — 41 finding(s), 2 candidate(s):
  - low `vendor/wheels/engineAdapters/Base.cfc` — declares prepareDIComplete()
  - low `vendor/wheels/engineAdapters/BoxLang/BoxLangAdapter.cfc` — declares prepareDIComplete()
- `method '$engineAdapter' in wheels.Global has no component return type → vendor/wheels/engineAdapters/Base.cfc` — 12 finding(s), 3 candidate(s):
  - low `vendor/wheels/engineAdapters/Base.cfc` — declares isLucee()
  - low `vendor/wheels/engineAdapters/Lucee/LuceeAdapter.cfc` — declares isLucee()
  - low `vendor/wheels/wheelstest/system/BaseSpec.cfc` — declares isLucee()
- `method 'getService' has no component return type → cli/lucli/services/Helpers.cfc` — 6 finding(s), 2 candidate(s):
  - low `cli/lucli/services/Helpers.cfc` — declares generateMigrationTimestamp()
  - low `cli/src/models/helpers.cfc` — declares generateMigrationTimestamp()
- `method 'getInstance' in wheels.Injector has no component return type → vendor/wheels/Dispatch.cfc` — 4 finding(s), 8 candidate(s):
  - low `vendor/wheels/Dispatch.cfc` — declares $init()
  - low `vendor/wheels/Mapper.cfc` — declares $init()
  - low `vendor/wheels/Plugins.cfc` — declares $init()
  - low `vendor/wheels/Public.cfc` — declares $init()
  - low `vendor/wheels/databaseAdapters/Base.cfc` — declares $init()
  - low `vendor/wheels/events/onapplicationstart.cfc` — declares $init()
  - low `vendor/wheels/interfaces/database/DatabaseModelAdapterInterface.cfc` — declares $init()
  - low `vendor/wheels/tests/_assets/dispatch/InvokeMethodFixture.cfc` — declares $init()
- `method '$classData' in author has no component return type → vendor/wheels/databaseAdapters/Base.cfc` — 1 finding(s), 6 candidate(s):
  - low `vendor/wheels/databaseAdapters/Base.cfc` — declares $supportsAdvisoryLocks()
  - low `vendor/wheels/databaseAdapters/CockroachDB/CockroachDBModel.cfc` — declares $supportsAdvisoryLocks()
  - low `vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerModel.cfc` — declares $supportsAdvisoryLocks()
  - low `vendor/wheels/databaseAdapters/MySQL/MySQLModel.cfc` — declares $supportsAdvisoryLocks()
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLModel.cfc` — declares $supportsAdvisoryLocks()
  - low `vendor/wheels/databaseAdapters/SQLite/SQLiteModel.cfc` — declares $supportsAdvisoryLocks()
- `method '$classData' in post has no component return type → vendor/wheels/databaseAdapters/Base.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/databaseAdapters/Base.cfc` — declares $isSharedModel()
  - low `vendor/wheels/interfaces/database/DatabaseModelAdapterInterface.cfc` — declares $isSharedModel()
- `method '$getObject' has no component return type → vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc` — declares key()
  - low `vendor/wheels/wheelstest/system/Assertion.cfc` — declares key()
- `method 'getJavaSystem' has no component return type → vendor/wheels/wheelstest/system/util/Env.cfc` — 1 finding(s), 4 candidate(s):
  - low `vendor/wheels/wheelstest/system/util/Env.cfc` — declares getEnv()
  - low `vendor/wheels/wheelstest/system/BaseSpec.cfc` — declares getEnv()
  - low `vendor/wheels/wheelstest/system/TestBox.cfc` — declares getEnv()
  - low `cli/lucli/tests/_modules/BaseModule.cfc` — declares getEnv()
- `method 'getLauncher' in wheels.wheelstest.BrowserClient has no component return type → vendor/wheels/tests/_assets/dispatch/InvokeMethodFixture.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/tests/_assets/dispatch/InvokeMethodFixture.cfc` — declares getState()
  - low `vendor/wheels/wheelstest/BrowserLauncher.cfc` — declares getState()
- `method 'getUtility' has no component return type → vendor/wheels/wheelstest/system/util/MixerUtil.cfc` — 1 finding(s), 9 candidate(s):
  - medium `vendor/wheels/wheelstest/system/util/MixerUtil.cfc` — declares start(); named like the receiver 'MixerUtil'
  - low `cli/lucli/Module.cfc` — declares start()
  - low `cli/lucli/services/rustcfml/RustCFMLEngine.cfc` — declares start()
  - low `cli/lucli/services/deploy/cli/DeployAccessoryCli.cfc` — declares start()
  - low `cli/lucli/services/deploy/cli/DeployAppCli.cfc` — declares start()
  - low `cli/lucli/services/deploy/cli/DeployProxyCli.cfc` — declares start()
  - low `cli/lucli/services/deploy/commands/AccessoryCommands.cfc` — declares start()
  - low `cli/lucli/services/deploy/commands/AppCommands.cfc` — declares start()
  - … 1 more
- `method 'policyScope' in Authorization has no component return type → vendor/wheels/Policy.cfc` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/Policy.cfc` — declares where()
  - low `vendor/wheels/model/query/QueryBuilder.cfc` — declares where()

</details>

## Method definitions — a method not found where it was looked for

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 49 | `model` | 1 | medium | declares model | `docs/presentations/cfug-2026-09-15/demo-app/seeds-with-comments.cfm:17` no qualifier, not in file |
| 12 | `flashinsert` | 2 | low | declares flashInsert (2 candidates) | `examples/starter-app/plugins/FlashMessagesBootstrap/index.cfm:6` no qualifier, not in file |
| 8 | `linkto` | 5 | low | declares linkTo (5 candidates) | `app/snippets/bootstrap/layout.cfm:50` no qualifier, not in file |
| 7 | `$appkey` | 1 | medium | declares $appKey | `vendor/wheels/Mapper.cfc:259` no qualifier, not in file |
| 6 | `csrfmetatags` | 1 | medium | declares csrfMetaTags | `app/snippets/bootstrap/layout.cfm:9` no qualifier, not in file |
| 6 | `flashmessages` | 2 | low | declares flashMessages (2 candidates) | `app/snippets/bootstrap/layout.cfm:55` no qualifier, not in file |
| 5 | `body` | 0 | none |  | `tools/article-tests/edge-cases.cfm:9` no qualifier, not in file |
| 5 | `includecontent` | 2 | low | declares includeContent (2 candidates) | `app/snippets/bootstrap/layout.cfm:57` no qualifier, not in file |
| 5 | `javascriptincludetag` | 1 | medium | declares javaScriptIncludeTag | `app/snippets/bootstrap/layout.cfm:65` no qualifier, not in file |
| 4 | `load` | 2 | low | declares load (2 candidates) | `cli/lucli/tests/specs/deploy/cli/DeployRegistryCliSpec.cfc:51` not found in extends chain |
| 4 | `mapper` | 1 | medium | declares mapper | `cli/lucli/templates/app/config/routes.cfm:7` no qualifier, not in file |
| 4 | `req` | 0 | none |  | `tools/article-tests/run.cfm:378` no qualifier, not in file |
| 4 | `seedonce` | 1 | medium | declares seedOnce | `docs/presentations/cfug-2026-09-15/demo-app/seeds-with-comments.cfm:5` no qualifier, not in file |
| 3 | `isdev` | 1 | medium | declares isDev | `vendor/wheels/tests/specs/buildInfoSpec.cfc:122` not found in extends chain |
| 3 | `issnapshot` | 1 | medium | declares isSnapshot | `vendor/wheels/tests/specs/buildInfoSpec.cfc:155` not found in extends chain |
| 3 | `mergescopemiddleware` | 0 | none |  | `tools/article-tests/run.cfm:332` no qualifier, not in file |
| 3 | `next` | 0 | none |  | `vendor/wheels/tests/_assets/middleware/TestMiddlewareA.cfc:16` no qualifier, not in file |
| 3 | `redirectto` | 2 | low | declares redirectTo (2 candidates) | `tools/vscode-ext/assets/templates/controller.cfc:36` not found in extends chain |
| 2 | `adaptername` | 7 | low | declares adapterName (7 candidates) | `vendor/wheels/tests/specs/migrator/MigratorOuterTransactionSpec.cfc:17` not found in extends chain |
| 2 | `authenticatethis` | 1 | medium | declares authenticateThis | `examples/starter-app/app/models/User.cfc:34` not found in extends chain |
| 2 | `config` | 164 | low | declares config; beside the calling file (164 candidates) | `vendor/wheels/Controller.cfc:61` not found in extends chain |
| 2 | `endformtag` | 2 | low | declares endFormTag (2 candidates) | `tools/vscode-ext/assets/templates/view-edit.cfm:9` no qualifier, not in file |
| 2 | `env` | 4 | low | declares env (4 candidates) | `cli/lucli/templates/app/config/settings.cfm:26` no qualifier, not in file |
| 2 | `errormessagesfor` | 1 | medium | declares errorMessagesFor | `tools/vscode-ext/assets/templates/view-edit.cfm:5` no qualifier, not in file |
| 2 | `getclassmetadata` | 0 | none |  | `vendor/wheels/wheelstest/system/TestBox.cfc:457` no qualifier, not in file |
| 2 | `getserverinfojson` | 0 | none |  | `cli/src/commands/wheels/benchmark.cfc:156` method 'getServerInfoJSON' not found in ServerService |
| 2 | `humanize` | 2 | low | declares humanize (2 candidates) | `cli/src/models/AdminIntrospectionService.cfc:333` method 'humanize' not found in helpers@wheels-cli |
| 2 | `includepartial` | 4 | low | declares includePartial (4 candidates) | `tools/vscode-ext/assets/templates/view-edit.cfm:7` no qualifier, not in file |
| 2 | `key` | 4 | low | declares key (3 candidates) | `tools/vscode-ext/assets/templates/controller.cfc:36` not found in extends chain |
| 2 | `renderview` | 2 | low | declares renderView (2 candidates) | `tools/vscode-ext/assets/templates/controller.cfc:39` not found in extends chain |
| 2 | `startformtag` | 2 | low | declares startFormTag (2 candidates) | `tools/vscode-ext/assets/templates/view-edit.cfm:6` no qualifier, not in file |
| 2 | `structvalues` | 0 | none |  | `vendor/wheels/public/mcp/SessionManager.cfc:103` no qualifier, not in file |
| 2 | `submittag` | 2 | low | declares submitTag (2 candidates) | `tools/vscode-ext/assets/templates/view-edit.cfm:8` no qualifier, not in file |
| 2 | `urlfor` | 6 | low | declares URLFor (6 candidates) | `vendor/wheels/public/layout/_navigation.cfm:28` no qualifier, not in file |
| 2 | `validatespresenceof` | 2 | low | declares validatesPresenceOf (2 candidates) | `examples/starter-app/plugins/authenticateThis/authenticateThis.cfc:36` no qualifier, not in file |
| 1 | `$$findmatchingroutes` | 1 | medium | declares $$findMatchingRoutes | `vendor/wheels/public/views/routetesterprocess.cfm:7` not found in extends chain |
| 1 | `$$pluginonlymethod` | 2 | low | declares $$pluginOnlyMethod (2 candidates) | `vendor/wheels/tests/specs/pluginsSpec.cfc:257` method '$$pluginOnlyMethod' not found in Test |
| 1 | `$componentintegrationplan` | 1 | medium | declares $componentIntegrationPlan | `vendor/wheels/Mapper.cfc:396` no qualifier, not in file |
| 1 | `$createobjectfromroot` | 1 | medium | declares $createObjectFromRoot | `cli/lucli/services/TestRunner.cfc:64` no qualifier, not in file |
| 1 | `$dbinfo` | 1 | medium | declares $dbinfo | `vendor/wheels/rocketunit_tests/env.cfm:67` no qualifier, not in file |
| 1 | `$encodefordisplaytext` | 1 | medium | declares $encodeForDisplayText | `vendor/wheels/events/onrequestend/complexity-panel.cfm:49` no qualifier, not in file |
| 1 | `$getdbtype` | 1 | medium | declares $getDBType | `vendor/wheels/global/strings.cfm:350` not found in extends chain |
| 1 | `$header` | 2 | low | declares $header (2 candidates) | `vendor/wheels/rocketunit_tests/env.cfm:28` no qualifier, not in file |
| 1 | `$helper01` | 2 | low | declares $helper01 (2 candidates) | `vendor/wheels/tests/specs/pluginsSpec.cfc:207` method '$helper01' not found in Test |
| 1 | `$resolveinjectedservices` | 1 | medium | declares $resolveInjectedServices | `vendor/wheels/Controller.cfc:118` not found in extends chain |
| 1 | `$setflashappend` | 1 | medium | declares $setFlashAppend | `vendor/wheels/Controller.cfc:57` not found in extends chain |
| 1 | `$setflashstorage` | 1 | medium | declares $setFlashStorage | `vendor/wheels/Controller.cfc:56` not found in extends chain |
| 1 | `beforevalidation` | 2 | low | declares beforeValidation (2 candidates) | `examples/starter-app/plugins/authenticateThis/authenticateThis.cfc:41` no qualifier, not in file |
| 1 | `binarymid` | 0 | none |  | `cli/tests/specs/e2e/ProjectScaffoldTest.cfc:261` not found in extends chain |
| 1 | `buttonto` | 3 | low | declares buttonTo (3 candidates) | `tools/vscode-ext/assets/templates/view-index.cfm:28` no qualifier, not in file |
| 1 | `columnnames` | 3 | low | declares columnNames (3 candidates) | `vendor/wheels/tests/specs/controller/SuperOverrideSpec.cfc:42` method 'columnNames' not found in superOverride |
| 1 | `command` | 1 | none |  | `cli/src/models/TestService.cfc:53` no qualifier, not in file |
| 1 | `controller` | 3 | low | declares controller (3 candidates) | `vendor/wheels/rocketunit_tests/env.cfm:57` no qualifier, not in file |
| 1 | `createapplicationsettings` | 1 | medium | declares createApplicationSettings | `examples/starter-app/app/events/onapplicationstart.cfm:11` no qualifier, not in file |
| 1 | `createslug` | 0 | none |  | `tools/vscode-ext/assets/templates/model.cfc:32` not found in extends chain |
| 1 | `elseif` | 0 | none |  | `vendor/wheels/Public.cfc:668` not found in extends chain |
| 1 | `exposemixin` | 1 | medium | declares exposeMixin | `vendor/wheels/wheelstest/system/BaseSpec.cfc:1578` method 'exposeMixin' not found in ANTJUnitReporter |
| 1 | `findone` | 4 | low | declares findOne (4 candidates) | `tools/vscode-ext/assets/templates/model.cfc:42` not found in extends chain |
| 1 | `fn` | 0 | none |  | `vendor/wheels/tests/specs/packages/LoadRegistryPackagesSpec.cfc:27` not found in extends chain |
| 1 | `fxreinclude` | 0 | none |  | `vendor/wheels/tests/specs/global/reloadGlobalsSpec.cfc:110` method 'fxReinclude' not found in wheels.Global |
| 1 | `getboxruntime` | 0 | none |  | `vendor/wheels/Test.cfc:804` not found in extends chain |
| 1 | `getcurrentdirectory` | 0 | none |  | `cli/src/models/EnvironmentService.cfc:1704` no qualifier, not in file |
| 1 | `getcwd` | 1 | none |  | `cli/src/models/MCPService.cfc:28` method 'getCWD' not found in fileSystem |
| 1 | `getnoninteractiveflag` | 0 | none |  | `cli/src/commands/wheels/base.cfc:168` method 'getNonInteractiveFlag' not found in commandbox.system.Shell |
| 1 | `haschanged` | 1 | medium | declares hasChanged | `vendor/wheels/tests/specs/model/callbacksSpec.cfc:554` not found in extends chain |
| 1 | `isauthenticated` | 1 | medium | declares isAuthenticated | `examples/starter-app/app/events/onrequeststart.cfm:7` no qualifier, not in file |
| 1 | `memoprobeinjected` | 0 | none |  | `vendor/wheels/tests/specs/global/promoteIncludedGlobalsMemoSpec.cfc:172` method 'memoProbeInjected' not found in wheels.tests._assets.global.PromoteMemoFixture |
| 1 | `println` | 2 | medium | declares println | `vendor/wheels/wheelstest/BrowserClient.cfc:417` no qualifier, not in file |
| 1 | `save` | 2 | low | declares save (2 candidates) | `tools/vscode-ext/assets/templates/controller.cfc:34` not found in extends chain |
| 1 | `setup` | 17 | low | declares setup (17 candidates) | `vendor/wheels/Test.cfc:323` not found in extends chain |
| 1 | `supercolumnnames` | 0 | none |  | `vendor/wheels/tests/_assets/models/SuperOverride.cfc:13` not found in extends chain |
| 1 | `teardown` | 13 | low | declares teardown (13 candidates) | `vendor/wheels/Test.cfc:362` not found in extends chain |
| 1 | `toargv` | 1 | medium | declares toArgv | `cli/lucli/tests/specs/services/ArgSpecSpec.cfc:195` not found in extends chain |
| 1 | `toconsole` | 1 | none |  | `cli/src/commands/wheels/docker/push.cfc:393` method 'toConsole' not found in DetailOutputService |
| 1 | `validatesconfirmationof` | 2 | low | declares validatesConfirmationOf (2 candidates) | `examples/starter-app/plugins/authenticateThis/authenticateThis.cfc:40` no qualifier, not in file |
| 1 | `validatesformatof` | 2 | low | declares validatesFormatOf (2 candidates) | `examples/starter-app/plugins/authenticateThis/authenticateThis.cfc:39` no qualifier, not in file |
| 1 | `variables[]` | 0 | none |  | `vendor/wheels/Test.cfc:788` not found in extends chain |

<details><summary>Groups with several candidates</summary>

- `flashinsert` — 12 finding(s), 2 candidate(s):
  - low `vendor/wheels/controller/flash.cfc:69` — declares flashInsert
  - low `vendor/wheels/interfaces/controller/ControllerFlashInterface.cfc:26` — declares flashInsert
- `linkto` — 8 finding(s), 5 candidate(s):
  - low `vendor/wheels/view/links.cfc:25` — declares linkTo
  - low `cli/lucli/tests/_helpers/RelatedBlockRenderer.cfc:21` — declares linkTo
  - low `examples/starter-app/plugins/jsconfirm/JSConfirm.cfc:11` — declares linkTo
  - low `vendor/wheels/interfaces/view/ViewLinkInterface.cfc:29` — declares linkTo
  - low `vendor/wheels/tests/_assets/controllers/SuperOverride.cfc:9` — declares linkTo
- `flashmessages` — 6 finding(s), 2 candidate(s):
  - low `vendor/wheels/view/miscellaneous.cfc:116` — declares flashMessages
  - low `examples/starter-app/plugins/FlashMessagesBootstrap/FlashMessagesBootstrap.cfc:8` — declares flashMessages
- `includecontent` — 5 finding(s), 2 candidate(s):
  - low `vendor/wheels/view/miscellaneous.cfc:326` — declares includeContent
  - low `vendor/wheels/interfaces/view/ViewContentInterface.cfc:32` — declares includeContent
- `load` — 4 finding(s), 2 candidate(s):
  - low `cli/lucli/services/deploy/config/ConfigLoader.cfc:38` — declares load
  - low `vendor/wheels/wheelstest/system/CodeComplexity.cfc:89` — declares load
- `redirectto` — 3 finding(s), 2 candidate(s):
  - low `vendor/wheels/controller/redirection.cfc:27` — declares redirectTo
  - low `vendor/wheels/interfaces/controller/ControllerRenderingInterface.cfc:115` — declares redirectTo
- `adaptername` — 2 finding(s), 7 candidate(s):
  - low `vendor/wheels/databaseAdapters/CockroachDB/CockroachDBMigrator.cfc:22` — declares adapterName
  - low `vendor/wheels/databaseAdapters/H2/H2Migrator.cfc:21` — declares adapterName
  - low `vendor/wheels/databaseAdapters/MicrosoftSQLServer/MicrosoftSQLServerMigrator.cfc:20` — declares adapterName
  - low `vendor/wheels/databaseAdapters/MySQL/MySQLMigrator.cfc:23` — declares adapterName
  - low `vendor/wheels/databaseAdapters/Oracle/OracleMigrator.cfc:27` — declares adapterName
  - low `vendor/wheels/databaseAdapters/PostgreSQL/PostgreSQLMigrator.cfc:21` — declares adapterName
  - low `vendor/wheels/databaseAdapters/SQLite/SQLiteMigrator.cfc:30` — declares adapterName
- `config` — 2 finding(s), 164 candidate(s):
  - low `vendor/wheels/Job.cfc:52` — declares config; beside the calling file
  - low `vendor/wheels/public/browser-fixtures/controllers/BrowserTestHome.cfc:14` — declares config
  - low `vendor/wheels/public/browser-fixtures/controllers/BrowserTestLogin.cfc:13` — declares config
  - low `vendor/wheels/rocketunit_tests/_assets/controllers/ApiTest.cfc:3` — declares config
  - low `vendor/wheels/rocketunit_tests/_assets/controllers/CsrfProtectedExcept.cfc:3` — declares config
  - low `vendor/wheels/rocketunit_tests/_assets/controllers/CsrfProtectedOnly.cfc:3` — declares config
  - low `vendor/wheels/rocketunit_tests/_assets/controllers/CsrfProtectedWithException.cfc:3` — declares config
  - low `vendor/wheels/rocketunit_tests/_assets/controllers/Filtering.cfc:3` — declares config
  - … 156 more
- `endformtag` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/view/forms.cfc:12` — declares endFormTag
  - low `vendor/wheels/interfaces/view/ViewFormInterface.cfc:60` — declares endFormTag
- `env` — 2 finding(s), 4 candidate(s):
  - low `cli/lucli/services/deploy/config/Accessory.cfc:62` — declares env
  - low `cli/lucli/services/deploy/config/Config.cfc:38` — declares env
  - low `cli/lucli/services/deploy/config/Role.cfc:25` — declares env
  - low `vendor/wheels/global/settings.cfm:39` — declares env
- `humanize` — 2 finding(s), 2 candidate(s):
  - low `cli/src/commands/wheels/generate/helper.cfc:407` — declares humanize
  - low `vendor/wheels/global/strings.cfm:134` — declares humanize
- `includepartial` — 2 finding(s), 4 candidate(s):
  - low `vendor/wheels/view/miscellaneous.cfc:291` — declares includePartial
  - low `vendor/wheels/interfaces/view/ViewContentInterface.cfc:44` — declares includePartial
  - low `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner01/Runner01.cfc:42` — declares includePartial
  - low `vendor/wheels/tests/_assets/plugins/runner/runner01/Runner01.cfc:42` — declares includePartial
- `key` — 2 finding(s), 3 candidate(s):
  - low `vendor/wheels/model/properties.cfm:206` — declares key
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc:66` — declares key
  - low `vendor/wheels/wheelstest/system/Assertion.cfc:416` — declares key
- `renderview` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/controller/rendering.cfc:18` — declares renderView
  - low `vendor/wheels/interfaces/controller/ControllerRenderingInterface.cfc:24` — declares renderView
- `startformtag` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/view/forms.cfc:48` — declares startFormTag
  - low `vendor/wheels/interfaces/view/ViewFormInterface.cfc:35` — declares startFormTag
- `submittag` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/view/forms.cfc:254` — declares submitTag
  - low `vendor/wheels/interfaces/view/ViewFormInterface.cfc:125` — declares submitTag
- `urlfor` — 2 finding(s), 6 candidate(s):
  - low `vendor/wheels/global/routing.cfm:471` — declares URLFor
  - low `vendor/wheels/interfaces/view/ViewLinkInterface.cfc:149` — declares urlFor
  - low `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner01/Runner01.cfc:7` — declares URLFor
  - low `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner02/Runner02.cfc:7` — declares URLFor
  - low `vendor/wheels/tests/_assets/plugins/runner/runner01/Runner01.cfc:7` — declares URLFor
  - low `vendor/wheels/tests/_assets/plugins/runner/runner02/Runner02.cfc:7` — declares URLFor
- `validatespresenceof` — 2 finding(s), 2 candidate(s):
  - low `vendor/wheels/model/validations.cfm:276` — declares validatesPresenceOf
  - low `vendor/wheels/interfaces/model/ModelValidationInterface.cfc:59` — declares validatesPresenceOf
- `$$pluginonlymethod` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/tests/_assets/plugins/runner/runner01/Runner01.cfc:17` — declares $$pluginOnlyMethod
  - low `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner01/Runner01.cfc:17` — declares $$pluginOnlyMethod
- `$header` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/global/tags.cfm:116` — declares $header
  - low `vendor/wheels/tests/_assets/events/OnErrorEventDouble.cfc:23` — declares $header
- `$helper01` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/tests/_assets/plugins/runner/runner01/Runner01.cfc:30` — declares $helper01
  - low `vendor/wheels/rocketunit_tests/_assets/plugins/runner/runner01/Runner01.cfc:30` — declares $helper01
- `beforevalidation` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/model/callbacks.cfm:178` — declares beforeValidation
  - low `vendor/wheels/interfaces/model/ModelCallbackInterface.cfc:19` — declares beforeValidation
- `buttonto` — 1 finding(s), 3 candidate(s):
  - low `vendor/wheels/view/links.cfc:164` — declares buttonTo
  - low `examples/starter-app/plugins/jsconfirm/JSConfirm.cfc:21` — declares buttonTo
  - low `vendor/wheels/interfaces/view/ViewLinkInterface.cfc:63` — declares buttonTo
- `columnnames` — 1 finding(s), 3 candidate(s):
  - low `vendor/wheels/model/miscellaneous.cfm:166` — declares columnNames
  - low `vendor/wheels/tests/_assets/models/SuperOverride.cfc:12` — declares columnNames
  - low `vendor/wheels/interfaces/model/ModelPropertyInterface.cfc:71` — declares columnNames
- `controller` — 1 finding(s), 3 candidate(s):
  - low `vendor/wheels/global/objects.cfm:520` — declares controller
  - low `vendor/wheels/mapper/scoping.cfc:159` — declares controller
  - low `vendor/wheels/interfaces/routing/RouteMapperInterface.cfc:227` — declares controller
- `findone` — 1 finding(s), 4 candidate(s):
  - low `vendor/wheels/model/read.cfm:475` — declares findOne
  - low `vendor/wheels/interfaces/model/ModelFinderInterface.cfc:80` — declares findOne
  - low `vendor/wheels/model/query/QueryBuilder.cfc:358` — declares findOne
  - low `vendor/wheels/model/query/ScopeChain.cfc:104` — declares findOne
- `save` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/model/create.cfm:85` — declares save
  - low `vendor/wheels/interfaces/model/ModelPersistenceInterface.cfc:53` — declares save
- `setup` — 1 finding(s), 17 candidate(s):
  - low `vendor/wheels/tests/specs/wheelstest/XUnitStyleLegacyTest.cfc:16` — declares setup
  - low `cli/lucli/Module.cfc:2236` — declares setup
  - low `cli/src/models/EnvironmentService.cfc:10` — declares setup
  - low `examples/starter-app/tests/RocketUnit/Test.cfc:23` — declares setup
  - low `cli/lucli/services/deploy/cli/DeployMainCli.cfc:274` — declares setup
  - low `cli/lucli/services/deploy/cli/DeployRegistryCli.cfc:24` — declares setup
  - low `examples/starter-app/plugins/authenticateThis/tests/AuthenticateTest.cfc:2` — declares setup
  - low `examples/starter-app/tests/RocketUnit/functions/Auth.cfc:4` — declares setup
  - … 9 more
- `teardown` — 1 finding(s), 13 candidate(s):
  - low `vendor/wheels/tests/specs/wheelstest/XUnitStyleLegacyTest.cfc:20` — declares teardown
  - low `examples/starter-app/tests/RocketUnit/Test.cfc:45` — declares teardown
  - low `examples/starter-app/plugins/authenticateThis/tests/AuthenticateTest.cfc:7` — declares teardown
  - low `examples/starter-app/tests/RocketUnit/functions/Auth.cfc:8` — declares teardown
  - low `examples/starter-app/tests/RocketUnit/functions/Permissions.cfc:7` — declares teardown
  - low `examples/starter-app/tests/RocketUnit/functions/Utils.cfc:6` — declares teardown
  - low `examples/starter-app/tests/RocketUnit/requests/Accounts.cfc:7` — declares teardown
  - low `examples/starter-app/tests/RocketUnit/requests/Main.cfc:7` — declares teardown
  - … 5 more
- `validatesconfirmationof` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/model/validations.cfm:74` — declares validatesConfirmationOf
  - low `vendor/wheels/interfaces/model/ModelValidationInterface.cfc:221` — declares validatesConfirmationOf
- `validatesformatof` — 1 finding(s), 2 candidate(s):
  - low `vendor/wheels/model/validations.cfm:129` — declares validatesFormatOf
  - low `vendor/wheels/interfaces/model/ModelValidationInterface.cfc:123` — declares validatesFormatOf

</details>

## Object references — a component path that names no file

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 21 | `base component does not resolve; 4 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/assets/clean.cfc:14` base component does not resolve; 4 inherited calls not checked |
| 20 | `base component does not resolve; 2 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/dbmigrate/info.cfc:4` base component does not resolve; 2 inherited calls not checked |
| 15 | `base component does not resolve; 5 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/cleanup/tmp.cfc:4` base component does not resolve; 5 inherited calls not checked |
| 14 | `base component does not resolve; 3 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/cleanup/logs.cfc:4` base component does not resolve; 3 inherited calls not checked |
| 10 | `base component does not resolve; 7 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/assets/precompile.cfc:15` base component does not resolve; 7 inherited calls not checked |
| 8 | `chained on 'mapper', which is not found (calling 'end')` | 4 | none |  | `cli/lucli/templates/app/config/routes.cfm:7` chained on 'mapper', which is not found (calling 'end') |
| 8 | `chained on 'mapper', which is not found (calling 'resources')` | 2 | none |  | `examples/starter-app/config/routes.cfm:7` chained on 'mapper', which is not found (calling 'resources') |
| 7 | `base component does not resolve; 1 inherited call not checked` | ? | none |  | `cli/src/commands/wheels/browser/install.cfc:13` base component does not resolve; 1 inherited call not checked |
| 7 | `base component does not resolve; 6 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/analyze/performance.cfc:8` base component does not resolve; 6 inherited calls not checked |
| 7 | `base component does not resolve; 8 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/config/diff.cfc:13` base component does not resolve; 8 inherited calls not checked |
| 7 | `chained on 'mapper', which is not found (calling 'post')` | 3 | none |  | `examples/starter-app/config/routes.cfm:7` chained on 'mapper', which is not found (calling 'post') |
| 6 | `base component does not resolve; 10 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/cleanup/sessions.cfc:4` base component does not resolve; 10 inherited calls not checked |
| 6 | `base component does not resolve; 11 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/config/check.cfc:12` base component does not resolve; 11 inherited calls not checked |
| 6 | `base component does not resolve; 9 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/assets/init.cfc:20` base component does not resolve; 9 inherited calls not checked |
| 5 | `chained on 'mapper', which is not found (calling 'root')` | 4 | none |  | `cli/lucli/templates/app/config/routes.cfm:7` chained on 'mapper', which is not found (calling 'root') |
| 5 | `component 'wheels.databaseAdapters.#local.adapterNamespace#.#local.adapterName#' does not exist (calling '$getColumns')` | 4 | none |  | `vendor/wheels/tests/specs/database/CockroachDBTypeSpec.cfc:92` component 'wheels.databaseAdapters.#local.adapterNamespace#.#local.adapterName#' does not exist (calling '$getColumns') |
| 4 | `base component does not resolve; 13 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/db/shell.cfc:10` base component does not resolve; 13 inherited calls not checked |
| 4 | `base component does not resolve; 15 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/db/setup.cfc:10` base component does not resolve; 15 inherited calls not checked |
| 4 | `base component does not resolve; 17 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/analyze/code.cfc:8` base component does not resolve; 17 inherited calls not checked |
| 4 | `chained on 'mapper', which is not found (calling 'put')` | 6 | none |  | `examples/starter-app/config/routes.cfm:7` chained on 'mapper', which is not found (calling 'put') |
| 3 | `chained on 'mapper', which is not found (calling 'wildcard')` | 2 | none |  | `cli/lucli/templates/app/config/routes.cfm:7` chained on 'mapper', which is not found (calling 'wildcard') |
| 2 | `base component does not resolve; 14 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/db/restore.cfc:10` base component does not resolve; 14 inherited calls not checked |
| 2 | `base component does not resolve; 16 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/generate/admin.cfc:14` base component does not resolve; 16 inherited calls not checked |
| 2 | `base component does not resolve; 18 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/env/setup.cfc:8` base component does not resolve; 18 inherited calls not checked |
| 2 | `base component does not resolve; 20 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/db/reset.cfc:10` base component does not resolve; 20 inherited calls not checked |
| 2 | `base component does not resolve; 24 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/deploy/setup.cfc:10` base component does not resolve; 24 inherited calls not checked |
| 2 | `base component does not resolve; 26 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/deploy/lock.cfc:10` base component does not resolve; 26 inherited calls not checked |
| 2 | `base component does not resolve; 34 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/db/dump.cfc:12` base component does not resolve; 34 inherited calls not checked |
| 2 | `base component does not resolve; 38 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/deploy/secrets.cfc:11` base component does not resolve; 38 inherited calls not checked |
| 2 | `chained on 'mapper', which is not found (calling 'scope')` | 6 | none |  | `examples/starter-app/config/routes.cfm:7` chained on 'mapper', which is not found (calling 'scope') |
| 2 | `chained on 'model', which is not found (calling 'findByKey')` | 3 | none |  | `tools/vscode-ext/assets/templates/controller.cfc:51` chained on 'model', which is not found (calling 'findByKey') |
| 2 | `component 'lucee.admin' does not exist (calling 'updateDatasource')` | 0 | low | file named admin.cfc (4 candidates) | `tools/ci/setup-datasources.cfm:7` component 'lucee.admin' does not exist (calling 'updateDatasource') |
| 2 | `extends DockerCommand, whose chain breaks at ../base, which does not resolve; 6 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/docker/login.cfc:9` extends DockerCommand, whose chain breaks at ../base, which does not resolve; 6 inherited calls not checked |
| 2 | `extends DockerCommand, whose chain breaks at ../base, which does not resolve; 7 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/docker/exec.cfc:11` extends DockerCommand, whose chain breaks at ../base, which does not resolve; 7 inherited calls not checked |
| 1 | `base component does not resolve; 12 inherited calls not checked` | ? | none |  | `examples/starter-app/tests/RocketUnit/requests/Accounts.cfc:1` base component does not resolve; 12 inherited calls not checked |
| 1 | `base component does not resolve; 29 inherited calls not checked` | ? | none |  | `examples/starter-app/tests/RocketUnit/functions/models/User.cfc:1` base component does not resolve; 29 inherited calls not checked |
| 1 | `base component does not resolve; 36 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/db/create.cfc:27` base component does not resolve; 36 inherited calls not checked |
| 1 | `base component does not resolve; 39 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/deploy/proxy.cfc:11` base component does not resolve; 39 inherited calls not checked |
| 1 | `base component does not resolve; 53 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/generate/app.cfc:36` base component does not resolve; 53 inherited calls not checked |
| 1 | `base component does not resolve; 56 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/deploy/push.cfc:10` base component does not resolve; 56 inherited calls not checked |
| 1 | `base component does not resolve; 61 inherited calls not checked` | ? | none |  | `cli/tests/specs/commands/ConfigCommandsTest.cfc:4` base component does not resolve; 61 inherited calls not checked |
| 1 | `base component does not resolve; 68 inherited calls not checked` | ? | none |  | `examples/starter-app/tests/RocketUnit/functions/Permissions.cfc:1` base component does not resolve; 68 inherited calls not checked |
| 1 | `base component does not resolve; 859 inherited calls not checked` | ? | none |  | `cli/lucli/Module.cfc:16` base component does not resolve; 859 inherited calls not checked |
| 1 | `calls a component whose chain breaks at ../base, which does not resolve; 2 calls not checked` | ? | none |  | `cli/src/models/helpers.cfc:30` calls a component whose chain breaks at ../base, which does not resolve; 2 calls not checked |
| 1 | `calls a component whose chain breaks at ../base, which does not resolve; 3 calls not checked` | ? | none |  | `cli/src/models/MCPService.cfc:164` calls a component whose chain breaks at ../base, which does not resolve; 3 calls not checked |
| 1 | `chained on 'getBoxRuntime', which is not found (calling 'executeStatement')` | 0 | none |  | `vendor/wheels/Test.cfc:804` chained on 'getBoxRuntime', which is not found (calling 'executeStatement') |
| 1 | `chained on 'mapper', which is not found (calling 'member')` | 2 | none |  | `examples/starter-app/config/routes.cfm:7` chained on 'mapper', which is not found (calling 'member') |
| 1 | `chained on 'mapper', which is not found (calling 'resource')` | 2 | none |  | `examples/starter-app/config/routes.cfm:7` chained on 'mapper', which is not found (calling 'resource') |
| 1 | `component '#arguments.bundlePath#' does not exist (calling 'getDebugBuffer')` | 2 | none |  | `vendor/wheels/wheelstest/system/TestBox.cfc:788` component '#arguments.bundlePath#' does not exist (calling 'getDebugBuffer') |
| 1 | `component 'modules.wheels.vendor.wheels.BuildInfo' does not exist (calling 'version')` | 6 | medium | file named BuildInfo.cfc | `cli/lucli/services/packages/PackagesMainCli.cfc:345` component 'modules.wheels.vendor.wheels.BuildInfo' does not exist (calling 'version') |
| 1 | `component 'wheels.databaseAdapters.#local.adapterNamespace#.#local.adapterName#' does not exist (calling '$getType')` | 7 | none |  | `vendor/wheels/tests/specs/database/CockroachDBTypeSpec.cfc:98` component 'wheels.databaseAdapters.#local.adapterNamespace#.#local.adapterName#' does not exist (calling '$getType') |
| 1 | `extends DockerCommand, whose chain breaks at ../base, which does not resolve; 13 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/docker/build.cfc:11` extends DockerCommand, whose chain breaks at ../base, which does not resolve; 13 inherited calls not checked |
| 1 | `extends DockerCommand, whose chain breaks at ../base, which does not resolve; 19 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/docker/push.cfc:12` extends DockerCommand, whose chain breaks at ../base, which does not resolve; 19 inherited calls not checked |
| 1 | `extends DockerCommand, whose chain breaks at ../base, which does not resolve; 26 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/docker/init.cfc:13` extends DockerCommand, whose chain breaks at ../base, which does not resolve; 26 inherited calls not checked |
| 1 | `extends DockerCommand, whose chain breaks at ../base, which does not resolve; 9 inherited calls not checked` | ? | none |  | `cli/src/commands/wheels/docker/deploy.cfc:11` extends DockerCommand, whose chain breaks at ../base, which does not resolve; 9 inherited calls not checked |

<details><summary>Groups with several candidates</summary>

- `component 'lucee.admin' does not exist (calling 'updateDatasource')` — 2 finding(s), 4 candidate(s):
  - low `cli/lucli/services/Admin.cfc` — file named admin.cfc
  - low `cli/src/commands/wheels/generate/admin.cfc` — file named admin.cfc
  - low `vendor/wheels/rocketunit_tests/_assets/controllers/admin/Admin.cfc` — file named admin.cfc
  - low `vendor/wheels/tests/_assets/controllers/admin/Admin.cfc` — file named admin.cfc

</details>
