# tb-p — unresolved findings and candidates

Every finding below is still reported. A candidate is a liberal match offered for a person to weigh, never an answer the resolver took: **high** is the only component declaring every method the function calls on the receiver (or the only one, named like it); **medium** is a sole match on weaker evidence, or the one named like the receiver among several; **low** is one of several. *Defined* is how many indexed files declare a method of that name: 0 means it is missing from the workspace, more means the resolver could not connect the call to it.

| Category | Findings | high | medium | low | none | method defined nowhere |
|---|---:|---:|---:|---:|---:|---:|
| variable | 158 | 19 | 39 | 9 | 91 | 67 |
| return-type | 27 | 5 | 18 | 1 | 3 | 3 |
| method | 63 | 0 | 48 | 4 | 11 | 11 |
| object | 10 | 0 | 0 | 0 | 10 | 8 |

## Variable definitions — a receiver whose component is unknown

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 23 | `controller → tests/resources/coldbox/system/FrameworkSupertype.cfc` | 2 | low | declares getSetting() (2 candidates) | `tests/resources/coldbox/system/FrameworkSupertype.cfc:19` variable 'controller' has no component ref |
| 20 | `arguments.actual` | 0 | none |  | `system/Assertion.cfc:1545` variable 'arguments.actual' has no component ref |
| 12 | `arguments.callbacks` | 0 | none |  | `system/TestBox.cfc:852` variable 'arguments.callbacks' has no component ref |
| 10 | `modulerecord.moduleconfig` | 1 | none |  | `system/TestBox.cfc:191` variable 'moduleRecord.moduleConfig' has no component ref |
| 8 | `response` | 0 | none |  | `system/TestBox.cfc:696` variable 'response' has no component ref |
| 7 | `arguments.baserunner → system/runners/BaseRunner.cfc` | 1 | high | declares canRunSuite(); named like the receiver 'baseRunner' | `system/TestBox.cfc:1112` variable 'arguments.baseRunner' has no component ref |
| 5 | `arguments.controller` | 1 | none |  | `tests/resources/coldbox/system/EventHandler.cfc:1` variable 'arguments.controller' has no component ref |
| 5 | `arguments.expected` | 0 | none |  | `system/Assertion.cfc:1742` variable 'arguments.expected' has no component ref |
| 5 | `arguments.runner` | 1 | none |  | `system/BaseSpec.cfc:1093` variable 'arguments.runner' has no component ref |
| 5 | `arguments.target` | 0 | none |  | `system/Assertion.cfc:2197` variable 'arguments.target' has no component ref |
| 5 | `controller` | 1 | none |  | `tests/resources/coldbox/system/FrameworkSupertype.cfc:96` variable 'controller' has no component ref |
| 4 | `arguments.receiver → system/MockBox.cfc` | 1 | medium | declares $property() | `system/compat/framework/TestCase.cfc:166` variable 'arguments.receiver' has no component ref |
| 4 | `arguments.spec` | 0 | none |  | `system/BaseSpec.cfc:1810` variable 'arguments.spec' has no component ref |
| 4 | `attributes.callbacks` | 0 | none |  | `system/runners/BDDRunner.cfc:265` variable 'attributes.callbacks' has no component ref |
| 4 | `thread.target → system/BaseSpec.cfc` | 1 | medium | declares runSpec() | `system/runners/BDDRunner.cfc:284` variable 'thread.target' has no component ref |
| 4 | `variables.spec → system/BaseSpec.cfc` | 1 | medium | declares expect() | `system/CollectionExpectation.cfc:119` variable 'variables.spec' has no component ref |
| 3 | `arguments.runner → system/TestBox.cfc` | 1 | high | declares announceToModules(); named like the receiver 'TestBox' | `system/BaseSpec.cfc:1129` variable 'arguments.runner' has no component ref |
| 3 | `arguments.runner → system/runners/BaseRunner.cfc` | 1 | high | declares canRunLabel(), canRunSpec() | `system/BaseSpec.cfc:1438` variable 'arguments.runner' has no component ref |
| 3 | `arguments.target → tests/specs/BDDInheritanceTest.cfc` | 39 | low | declares run(), beforeAll(), afterAll() (19 candidates) | `system/runners/BDDRunner.cfc:51` variable 'arguments.target' has no component ref |
| 3 | `attributes.service → system/util/StreamingService.cfc` | 1 | high | declares queueEvent(), streamEvent() | `tests/specs/streaming/StreamingServiceTest.cfc:452` variable 'attributes.service' has no component ref |
| 3 | `spec` | 0 | none |  | `tests/specs/BDDLifecycleTest.cfc:34` variable 'spec' has no component ref |
| 2 | `arguments.suite → system/BaseSpec.cfc` | 1 | medium | declares beforeEach() | `system/BaseSpec.cfc:1259` variable 'arguments.suite' has no component ref |
| 2 | `nextclosure` | 0 | none |  | `system/BaseSpec.cfc:1357` variable 'nextClosure' has no component ref |
| 2 | `this.mockbox → system/MockBox.cfc` | 1 | high | declares normalizeArguments(); named like the receiver 'mockBox' | `system/MockBox.cfc:400` variable 'this.mockBox' has no component ref |
| 2 | `wirebox` | 0 | none |  | `tests/resources/coldbox/system/FrameworkSupertype.cfc:80` variable 'wirebox' has no component ref |
| 1 | `arguments.targetobject → system/mockutils/MockGenerator.cfc` | 1 | medium | declares $include() | `system/mockutils/MockGenerator.cfc:276` variable 'arguments.targetObject' has no component ref |
| 1 | `config.moduleconfig → tests/resources/testModule/ModuleConfig.cfc` | 1 | high | declares onUnload(); named like the receiver 'moduleConfig' | `system/TestBox.cfc:504` variable 'config.moduleConfig' has no component ref |
| 1 | `controller → tests/resources/coldbox/system/EventHandler.cfc` | 2 | medium | declares getWirebox() | `tests/resources/coldbox/system/FrameworkSupertype.cfc:45` variable 'controller' has no component ref |
| 1 | `idata.type → system/reports/ANTJUnitReporter.cfc` | 16 | low | declares runReport() (16 candidates) | `system/TestBox.cfc:747` variable 'iData.type' has no component ref |
| 1 | `item → system/BaseSpec.cfc` | 1 | medium | declares beforeEach() | `system/BaseSpec.cfc:1255` variable 'item' has no component ref |
| 1 | `obj2 → tests/resources/CallPrivate.cfc` | 1 | medium | declares callIt() | `tests/specs/MXUnitCompatTest.cfc:173` variable 'obj2' has no component ref |
| 1 | `omockgenerator → system/mockutils/MockGenerator.cfc` | 1 | high | declares generate(); named like the receiver 'oMockGenerator' | `system/MockBox.cfc:480` variable 'oMockGenerator' has no component ref |
| 1 | `parentsuite → system/BaseSpec.cfc` | 1 | medium | declares afterEach() | `system/BaseSpec.cfc:1402` variable 'parentSuite' has no component ref |
| 1 | `runner → system/runners/BaseRunner.cfc` | 1 | medium | declares isSuiteFocused() | `system/BaseSpec.cfc:1080` variable 'runner' has no component ref |
| 1 | `variables.logbox` | 1 | none |  | `tests/resources/coldbox/system/EventHandler.cfc:1` variable 'variables.logBox' has no component ref |

<details><summary>Groups with several candidates</summary>

- `controller → tests/resources/coldbox/system/FrameworkSupertype.cfc` — 23 finding(s), 2 candidate(s):
  - low `tests/resources/coldbox/system/FrameworkSupertype.cfc` — declares getSetting()
  - low `tests/resources/Test.cfc` — declares getSetting()
- `arguments.target → tests/specs/BDDInheritanceTest.cfc` — 3 finding(s), 19 candidate(s):
  - low `tests/specs/BDDInheritanceTest.cfc` — declares run(), beforeAll(), afterAll()
  - low `tests/specs/BDDLifecycleAnnotationsTest.cfc` — declares run(), beforeAll(), afterAll()
  - low `tests/specs/BDDLifecycleTest.cfc` — declares run(), beforeAll(), afterAll()
  - low `tests/specs/BDDTest.cfc` — declares run(), beforeAll(), afterAll()
  - low `tests/specs/CustomAssertions.cfc` — declares run(), beforeAll(), afterAll()
  - low `tests/specs/DebugTests.cfc` — declares run(), beforeAll(), afterAll()
  - low `tests/specs/ExcludesTest.cfc` — declares run(), beforeAll(), afterAll()
  - low `tests/specs/FocusedSpecs.cfc` — declares run(), beforeAll(), afterAll()
  - … 11 more
- `idata.type → system/reports/ANTJUnitReporter.cfc` — 1 finding(s), 16 candidate(s):
  - low `system/reports/ANTJUnitReporter.cfc` — declares runReport()
  - low `system/reports/CodexWikiReporter.cfc` — declares runReport()
  - low `system/reports/ConsoleReporter.cfc` — declares runReport()
  - low `system/reports/DocReporter.cfc` — declares runReport()
  - low `system/reports/DotReporter.cfc` — declares runReport()
  - low `system/reports/IReporter.cfc` — declares runReport()
  - low `system/reports/JSONReporter.cfc` — declares runReport()
  - low `system/reports/JUnitReporter.cfc` — declares runReport()
  - … 8 more

</details>

## Return types — a call chained on a method that declares no component

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 8 | `method 'getConsoleUtil' has no component return type → system/util/ConsoleUtil.cfc` | 2 | medium | declares color(); named like the receiver 'ConsoleUtil' (2 candidates) | `system/reports/BaseReporter.cfc:37` method 'getConsoleUtil' has no component return type (chain to 'color') |
| 4 | `method 'getUtility' has no component return type → system/util/Util.cfc` | 1 | medium | declares getAnnotatedMethods() | `system/BaseSpec.cfc:1238` method 'getUtility' has no component return type (chain to 'getAnnotatedMethods') |
| 4 | `method 'withContext' in testbox.system.Expectation has no component return type → system/Expectation.cfc` | 1 | medium | declares toBe() | `tests/specs/BDDTest.cfc:366` method 'withContext' in testbox.system.Expectation has no component return type (chain to 'toBe') |
| 3 | `method 'getJavaSystem' has no component return type → system/BaseSpec.cfc` | 1 | high | declares getProperty(), getEnv() | `system/util/Env.cfc:15` method 'getJavaSystem' has no component return type (chain to 'getProperty') |
| 1 | `method 'getCBMockData' has no component return type → system/compat/framework/TestCase.cfc` | 1 | medium | declares mock() | `system/BaseSpec.cfc:1684` method 'getCBMockData' has no component return type (chain to 'mock') |
| 1 | `method 'getInstance' has no component return type → tests/resources/coldbox/system/FrameworkSupertype.cfc` | 1 | medium | declares addAsset() | `tests/resources/coldbox/system/FrameworkSupertype.cfc:393` method 'getInstance' has no component return type (chain to 'addAsset') |
| 1 | `method 'getJavaSystem' has no component return type → system/util/Env.cfc` | 4 | low | declares getEnv() (3 candidates) | `system/util/Env.cfc:68` method 'getJavaSystem' has no component return type (chain to 'getEnv') |
| 1 | `method 'getUtility' has no component return type → system/util/MixerUtil.cfc` | 1 | high | declares start(); named like the receiver 'MixerUtil' | `system/BaseSpec.cfc:1643` method 'getUtility' has no component return type (chain to 'start') |
| 1 | `method 'makePublic' has no component return type` | 0 | none |  | `tests/specs/MXUnitCompatTest.cfc:169` method 'makePublic' has no component return type (chain to 'funkyMethod') |
| 1 | `method 'makePublic' has no component return type → tests/resources/test1.cfc` | 1 | medium | declares aPrivateMethod() | `tests/specs/MXUnitCompatTest.cfc:166` method 'makePublic' has no component return type (chain to 'aPrivateMethod') |
| 1 | `method 'withContext' in Expectation has no component return type` | 0 | none |  | `tests/specs/BDDTest.cfc:410` method 'withContext' in Expectation has no component return type (chain to 'toBeGoofy') |
| 1 | `method 'withContext' in testbox.system.Expectation has no component return type` | 0 | none |  | `tests/specs/BDDTest.cfc:393` method 'withContext' in testbox.system.Expectation has no component return type (chain to 'notToBe') |

<details><summary>Groups with several candidates</summary>

- `method 'getConsoleUtil' has no component return type → system/util/ConsoleUtil.cfc` — 8 finding(s), 2 candidate(s):
  - medium `system/util/ConsoleUtil.cfc` — declares color(); named like the receiver 'ConsoleUtil'
  - low `system/reports/BaseReporter.cfc` — declares color()
- `method 'getJavaSystem' has no component return type → system/util/Env.cfc` — 1 finding(s), 3 candidate(s):
  - low `system/util/Env.cfc` — declares getEnv()
  - low `system/BaseSpec.cfc` — declares getEnv()
  - low `system/TestBox.cfc` — declares getEnv()

</details>

## Method definitions — a method not found where it was looked for

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 41 | `getstringname` | 1 | medium | declares getStringName; beside the calling file | `system/Expectation.cfc:421` no qualifier, not in file |
| 3 | `assertisfunky` | 1 | medium | declares assertIsFunky | `tests/specs/CustomAssertions.cfc:30` method 'assertIsFunky' not found in testbox.system.Assertion |
| 3 | `getclassmetadata` | 0 | none |  | `system/TestBox.cfc:460` no qualifier, not in file |
| 2 | `assertisawesome` | 1 | medium | declares assertIsAwesome | `tests/specs/CustomAssertions.cfc:26` method 'assertIsAwesome' not found in testbox.system.Assertion |
| 2 | `fail` | 4 | low | declares fail (4 candidates) | `tests/resources/CustomAsserts.cfc:4` no qualifier, not in file |
| 1 | `body` | 0 | none |  | `system/Expectation.cfc:48` no qualifier, not in file |
| 1 | `datanavigate` | 0 | none |  | `system/Assertion.cfc:2242` no qualifier, not in file |
| 1 | `exposemixin` | 1 | medium | declares exposeMixin | `system/BaseSpec.cfc:1645` method 'exposeMixin' not found in test1 |
| 1 | `exxpect` | 0 | none |  | `tests/specs/BDDTest.cfc:844` not found in extends chain |
| 1 | `getoptions` | 3 | low | declares getOptions (3 candidates) | `system/compat/runner/Results.cfc:62` no qualifier, not in file |
| 1 | `invalidfunction` | 0 | none |  | `tests/specs/AssertionsTest.cfc:28` not found in extends chain |
| 1 | `isawesome` | 0 | none |  | `tests/specs/AssertionsTest.cfc:64` method 'isAwesome' not found in testbox.system.Assertion |
| 1 | `isboxset` | 0 | none |  | `system/MockBox.cfc:698` no qualifier, not in file |
| 1 | `isnotawesome` | 0 | none |  | `tests/specs/AssertionsTest.cfc:68` method 'isNotAwesome' not found in testbox.system.Assertion |
| 1 | `isrange` | 1 | medium | declares isRange; beside the calling file | `system/MockBox.cfc:689` no qualifier, not in file |
| 1 | `run` | 39 | low | declares run (37 candidates) | `cfml/browser/index.cfm:29` no qualifier, not in file |
| 1 | `virtualreturn` | 0 | none |  | `tests/specs/mockbox/MockBoxTestHarness.cfm:33` method 'virtualReturn' not found in testbox.tests.resources.Test |

<details><summary>Groups with several candidates</summary>

- `fail` — 2 finding(s), 4 candidate(s):
  - low `system/Assertion.cfc:15` — declares fail
  - low `system/BaseSpec.cfc:75` — declares fail
  - low `system/Expectation.cfc:60` — declares fail
  - low `system/compat/framework/TestCase.cfc:199` — declares fail
- `getoptions` — 1 finding(s), 3 candidate(s):
  - low `system/TestBox.cfc:25` — declares getOptions
  - low `system/runners/BDDRunner.cfc:14` — declares getOptions
  - low `system/runners/UnitRunner.cfc:14` — declares getOptions
- `run` — 1 finding(s), 37 candidate(s):
  - low `build/Build.cfc:53` — declares run
  - low `cfml/tests/specs/BDDTest.cfc:31` — declares run
  - low `cfml/tests/specs/MyFirstSpec.cfc:3` — declares run
  - low `system/TestBox.cfc:333` — declares run
  - low `system/runners/BDDRunner.cfc:38` — declares run
  - low `system/runners/IRunner.cfc:24` — declares run
  - low `system/runners/UnitRunner.cfc:38` — declares run
  - low `tests/specs/AbstractClass.cfc:8` — declares run
  - … 29 more

</details>

## Object references — a component path that names no file

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 4 | `component 'testbox.system.modules.globber.models.PathPatternMatcher' does not exist (calling 'matchPatterns')` | 0 | none |  | `system/TestBox.cfc:965` component 'testbox.system.modules.globber.models.PathPatternMatcher' does not exist (calling 'matchPatterns') |
| 1 | `component '#arguments.bundlePath#' does not exist (calling 'getDebugBuffer')` | 1 | none |  | `system/TestBox.cfc:883` component '#arguments.bundlePath#' does not exist (calling 'getDebugBuffer') |
| 1 | `component '#arguments.bundlePath#' does not exist (calling 'run')` | 39 | none |  | `system/TestBox.cfc:1041` component '#arguments.bundlePath#' does not exist (calling 'run') |
| 1 | `component 'CFIDE.componentutils.cfcexplorer' does not exist (calling 'getcfcs')` | 0 | none |  | `system/compat/runner/DirectoryTestSuite.cfc:49` component 'CFIDE.componentutils.cfcexplorer' does not exist (calling 'getcfcs') |
| 1 | `component 'CFIDE.componentutils.cfcexplorer' does not exist (calling 'normalizePath')` | 0 | none |  | `system/compat/runner/DirectoryTestSuite.cfc:48` component 'CFIDE.componentutils.cfcexplorer' does not exist (calling 'normalizePath') |
| 1 | `component 'testbox.system.modules.cbstreams.models.StreamBuilder' does not exist (chain hop 'new' to 'peek')` | 0 | none |  | `system/coverage/browser/CodeBrowser.cfc:62` component 'testbox.system.modules.cbstreams.models.StreamBuilder' does not exist (chain hop 'new' to 'peek') |
| 1 | `component 'testbox.system.modules.cbstreams.models.StreamBuilder' does not exist (chain hop 'new' to 'rangeClosed')` | 0 | none |  | `system/coverage/browser/CodeBrowser.cfc:62` component 'testbox.system.modules.cbstreams.models.StreamBuilder' does not exist (chain hop 'new' to 'rangeClosed') |
