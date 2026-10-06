# cx-p — unresolved findings and candidates

Every finding below is still reported. A candidate is a liberal match offered for a person to weigh, never an answer the resolver took: **high** is the only component declaring every method the function calls on the receiver (or the only one, named like it); **medium** is a sole match on weaker evidence, or the one named like the receiver among several; **low** is one of several. *Defined* is how many indexed files declare a method of that name: 0 means it is missing from the workspace, more means the resolver could not connect the call to it.

| Category | Findings | high | medium | low | none | method defined nowhere |
|---|---:|---:|---:|---:|---:|---:|
| variable | 1016 | 472 | 186 | 98 | 260 | 117 |
| return-type | 157 | 4 | 52 | 60 | 41 | 37 |
| method | 92 | 0 | 47 | 17 | 28 | 20 |
| object | 51 | 0 | 0 | 5 | 46 | 15 |

## Variable definitions — a receiver whose component is unknown

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 70 | `oexception → system/web/context/ExceptionBean.cfc` | 1 | high | declares getErrorCode(), getExtraMessage(), getType(), getExceptionStruct(), getMessage() | `system/exceptions/BugReport-Public.cfm:15` variable 'oException' has no component ref |
| 46 | `arguments.prc.response → system/web/context/Response.cfc` | 1 | high | declares setFormat(), addHeader(), setResponseTime(), getDataPacket(), getError(), getFormat(), getContentType(), getStatusCode(), getLocation(), getBinary()… | `system/RestHandler.cfc:55` variable 'arguments.prc.response' has no component ref |
| 45 | `now` | 0 | none |  | `system/async/tasks/ScheduledTask.cfc:586` variable 'now' has no component ref |
| 41 | `mapping → system/ioc/config/Mapping.cfc` | 1 | high | declares isAspect(), getName(); named like the receiver 'mapping' | `system/aop/Mixer.cfc:104` variable 'mapping' has no component ref |
| 39 | `pool → system/core/events/EventPool.cfc` | 4 | high | declares register(), exists(), getObject(), unregister(), process(), getListenerChain() | `tests/specs/core/events/EventPoolTest.cfc:32` variable 'pool' has no component ref |
| 30 | `arguments.ehbean → system/web/context/EventHandlerBean.cfc` | 1 | high | declares getModule(), isModule(), getRunnable() | `system/web/services/HandlerService.cfc:103` variable 'arguments.ehBean' has no component ref |
| 26 | `results.config` | 1 | none |  | `system/web/services/ModuleService.cfc:1295` variable 'results.config' has no component ref |
| 25 | `results.ehbean → system/web/context/EventHandlerBean.cfc` | 1 | high | declares getViewDispatch(), getActionMetadata(), getMethod(), isMissingAction(), getMissingAction() | `system/web/Controller.cfc:826` variable 'results.ehBean' has no component ref |
| 24 | `variables.router → system/web/routing/Router.cfc` | 1 | high | declares isValidExtension(), getMimeExtensionAlias(), route(), getRoutes(); named like the receiver 'router' | `tests/specs/web/routing/RouterSSETest.cfc:49` variable 'variables.router' has no component ref |
| 23 | `router` | 1 | none |  | `tests/specs/web/routing/ResourcesTest.cfc:12` variable 'router' has no component ref |
| 22 | `cacheconfig` | 0 | none |  | `system/cache/providers/CFProvider.cfc:503` variable 'cacheConfig' has no component ref |
| 20 | `e` | 1 | none |  | `tests/specs/integration/EventExecutionsSpec.cfc:16` variable 'e' has no component ref |
| 18 | `arguments.mapping → system/ioc/config/Mapping.cfc` | 2 | medium | declares getObjectMetadata(); named like the receiver 'mapping' (2 candidates) | `system/aop/Mixer.cfc:215` variable 'arguments.mapping' has no component ref |
| 17 | `orequestcontext → system/web/context/RequestContext.cfc` | 1 | high | declares getSESBaseURL(); named like the receiver 'oRequestContext' | `system/web/Controller.cfc:544` variable 'oRequestContext' has no component ref |
| 15 | `application[] → system/web/Controller.cfc` | 1 | high | declares getLoaderService(), getSetting(), runEvent(), getLogBox() | `system/Bootstrap.cfc:101` variable 'application[]' has no component ref |
| 15 | `ohandler` | 1 | none |  | `system/web/Controller.cfc:859` variable 'oHandler' has no component ref |
| 15 | `scope → system/ioc/scopes/Singleton.cfc` | 1 | high | declares getSingletons(), getFromScope() | `tests/specs/ioc/scopes/SingletonTest.cfc:22` variable 'scope' has no component ref |
| 14 | `arguments.now` | 0 | none |  | `system/async/tasks/ScheduledTask.cfc:1682` variable 'arguments.now' has no component ref |
| 14 | `arguments.oconfig → system/core/dynamic/MixerUtil.cfc` | 1 | medium | declares getPropertyMixin() | `system/web/config/ApplicationLoader.cfc:233` variable 'arguments.oConfig' has no component ref |
| 14 | `variables.native → system/async/executors/Executor.cfc` | 1 | medium | declares isTerminated() | `system/async/executors/Executor.cfc:175` variable 'variables.native' has no component ref |
| 13 | `buffer → system/web/context/InterceptorBuffer.cfc` | 1 | high | declares hasContent(), getString(), length() | `tests/specs/web/context/InterceptorBufferTest.cfc:30` variable 'buffer' has no component ref |
| 13 | `event1 → system/web/context/RequestContext.cfc` | 1 | medium | declares getPrivateCollection() | `tests/specs/integration/EventCachingSpec.cfc:41` variable 'event1' has no component ref |
| 12 | `cachestats → system/cache/util/CacheStats.cfc` | 5 | medium | declares getHits(), getMisses(), getGarbageCollections(), getEvictionCount(); named like the receiver 'cacheStats' (5 candidates) | `system/cache/report/skins/default/CacheCharting.cfm:37` variable 'cacheStats' has no component ref |
| 12 | `variables.asyncmanager → system/async/AsyncManager.cfc` | 5 | medium | declares out(); named like the receiver 'asyncManager' (5 candidates) | `system/async/tasks/Scheduler.cfc:153` variable 'variables.asyncManager' has no component ref |
| 11 | `e → system/testing/BaseTestCase.cfc` | 1 | high | declares getHandlerResults(), getRenderedContent(), getRenderData() | `tests/specs/integration/RenderingsSpec.cfc:16` variable 'e' has no component ref |
| 11 | `expectation.actual → system/web/context/Response.cfc` | 2 | high | declares getStatusCode(), getMemento(), getData() | `system/testing/CustomMatchers.cfc:64` variable 'expectation.actual' has no component ref |
| 11 | `prc.response → system/web/context/Response.cfc` | 1 | medium | declares addMessage() | `system/RestHandler.cfc:220` variable 'prc.response' has no component ref |
| 10 | `emitter → system/web/context/SSEEmitter.cfc` | 2 | low | declares send(), isClosed() (2 candidates) | `system/web/routing/Router.cfc:2616` variable 'emitter' has no component ref |
| 10 | `event2 → system/web/context/RequestContext.cfc` | 1 | medium | declares getPrivateCollection() | `tests/specs/integration/EventCachingSpec.cfc:52` variable 'event2' has no component ref |
| 10 | `obean → test-harness/models/formBean.cfc` | 1 | high | declares getFname(), getLname() | `tests/specs/FrameworkSuperTypeTest.cfc:70` variable 'oBean' has no component ref |
| 10 | `oscheduler` | 21 | none |  | `system/web/services/SchedulerService.cfc:123` variable 'oScheduler' has no component ref |
| 9 | `scope → system/ioc/scopes/CFScopes.cfc` | 6 | low | declares getFromScope() (6 candidates) | `tests/specs/ioc/scopes/CFScopesTest.cfc:51` variable 'scope' has no component ref |
| 8 | `caches.cache1` | 1 | none |  | `tests/specs/cache/CacheFactoryTest.cfc:62` variable 'caches.cache1' has no component ref |
| 7 | `arguments.requestcontext → system/web/context/RequestContext.cfc` | 1 | high | declares getCurrentRouteRecord(); named like the receiver 'requestContext' | `system/web/services/HandlerService.cfc:579` variable 'arguments.requestContext' has no component ref |
| 7 | `arguments.target → system/async/time/Duration.cfc` | 2 | high | declares plusDays(), plusHours(), plusMinutes(), plusSeconds(), plusNanos() | `system/async/time/DateTimeHelper.cfc:436` variable 'arguments.target' has no component ref |
| 7 | `exception → system/web/context/ExceptionBean.cfc` | 2 | high | declares getType(), getMessage(), getDetail(), getExtendedInfo(), getTagContext(), getStackTrace(); named like the receiver 'exception' | `test-harness/views/_templates/generic_error.cfm:12` variable 'exception' has no component ref |
| 7 | `services.handlerservice → system/web/services/HandlerService.cfc` | 1 | high | declares getHandlerBean(), getHandler(); named like the receiver 'handlerService' | `system/web/Controller.cfc:821` variable 'services.handlerService' has no component ref |
| 7 | `services.interceptorservice → system/web/services/InterceptorService.cfc` | 5 | medium | declares announce(); named like the receiver 'interceptorService' (5 candidates) | `system/web/Controller.cfc:570` variable 'services.interceptorService' has no component ref |
| 7 | `services.requestservice → system/web/services/RequestService.cfc` | 3 | high | declares getContext(), getFlashScope(); named like the receiver 'requestService' | `system/web/Controller.cfc:459` variable 'services.requestService' has no component ref |
| 6 | `caches.cache2` | 1 | none |  | `tests/specs/cache/CacheFactoryTest.cfc:63` variable 'caches.cache2' has no component ref |
| 6 | `oappender → system/logging/appenders/FileAppender.cfc` | 5 | high | declares getProperty(), getLogFullPath(), getName(), getlockTimeout(), initLoglocation() | `system/logging/util/FileRotator.cfc:24` variable 'oAppender' has no component ref |
| 6 | `variables.executor → system/async/executors/Executor.cfc` | 5 | medium | declares getNative(); named like the receiver 'executor' (5 candidates) | `system/async/tasks/Future.cfc:66` variable 'variables.executor' has no component ref |
| 5 | `arguments.target → system/core/dynamic/MixerUtil.cfc` | 1 | medium | declares injectPropertyMixin() | `system/core/dynamic/ObjectPopulator.cfc:417` variable 'arguments.target' has no component ref |
| 5 | `expectation.actual → system/web/context/RequestContext.cfc` | 2 | low | declares getStatusCode(), getMemento() (2 candidates) | `system/testing/CustomMatchers.cfc:29` variable 'expectation.actual' has no component ref |
| 5 | `request.testbox → system/logging/Logger.cfc` | 5 | low | declares debug() (3 candidates) | `system/testing/CustomMatchers.cfc:32` variable 'request.testbox' has no component ref |
| 5 | `variables.emitter → system/web/context/SSEEmitter.cfc` | 2 | low | declares isClosed() (2 candidates) | `system/web/context/SSEEmitter.cfc:60` variable 'variables.emitter' has no component ref |
| 4 | `arguments.entity` | 0 | none |  | `test-harness/models/entities/BoxLangEventHandler.cfc:33` variable 'arguments.entity' has no component ref |
| 4 | `arguments.item` | 2 | none |  | `tests/specs/async/AsyncManagerSpec.cfc:302` variable 'arguments.item' has no component ref |
| 4 | `arguments.target` | 1 | none |  | `system/async/time/DateTimeHelper.cfc:78` variable 'arguments.target' has no component ref |
| 4 | `arguments.targetobject → system/core/dynamic/MixerUtil.cfc` | 1 | medium | declares injectMixin() | `system/ioc/Injector.cfc:1185` variable 'arguments.targetObject' has no component ref |
| 4 | `mconfig.injector → system/ioc/Injector.cfc` | 2 | high | declares getBinder(), registerNewInstance(), getInstance(); named like the receiver 'injector' | `system/web/services/ModuleService.cfc:726` variable 'mConfig.injector' has no component ref |
| 4 | `mconfig.router → system/web/routing/Router.cfc` | 1 | high | declares configure(), getRoutes(), route(); named like the receiver 'router' | `system/web/services/ModuleService.cfc:849` variable 'mConfig.router' has no component ref |
| 4 | `my.out` | 0 | none |  | `system/remote/RemotingUtil.cfc:29` variable 'my.out' has no component ref |
| 4 | `nextrun → system/async/time/Duration.cfc` | 1 | medium | declares plusHours() | `system/async/tasks/ScheduledTask.cfc:1052` variable 'nextRun' has no component ref |
| 4 | `results.injector → system/ioc/Injector.cfc` | 2 | high | declares setParent(), getInstance(), getBinder(); named like the receiver 'injector' | `system/web/services/ModuleService.cfc:1259` variable 'results.injector' has no component ref |
| 4 | `runnableinstance` | 83 | none |  | `system/web/routing/Router.cfc:2573` variable 'runnableInstance' has no component ref |
| 4 | `services[] → system/web/services/BaseService.cfc` | 7 | low | declares onConfigurationLoad(), afterAspectsLoad() (2 candidates) | `system/web/services/LoaderService.cfc:73` variable 'services[]' has no component ref |
| 4 | `stats` | 0 | none |  | `tests/tmp/ormTest.cfm:12` variable 'stats' has no component ref |
| 4 | `target` | 0 | none |  | `system/async/time/DateTimeHelper.cfc:313` variable 'target' has no component ref |
| 3 | `application[] → system/FrameworkSupertype.cfc` | 2 | low | declares getSetting(), getLog() (2 candidates) | `system/Bootstrap.cfc:517` variable 'application[]' has no component ref |
| 3 | `args.target → system/core/dynamic/MixerUtil.cfc` | 1 | high | declares injectPropertyMixin(), injectMixin() | `system/ioc/Builder.cfc:1115` variable 'args.target' has no component ref |
| 3 | `arguments.context → system/web/context/RequestContext.cfc` | 1 | high | declares getCurrentEvent(), removeEventCacheableEntry(), setEventCacheableEntry() | `system/web/services/RequestService.cfc:152` variable 'arguments.context' has no component ref |
| 3 | `arguments.logboxconfig → system/cache/config/CacheBoxConfig.cfc` | 2 | low | declares validate() (2 candidates) | `system/web/config/ApplicationLoader.cfc:851` variable 'arguments.logBoxConfig' has no component ref |
| 3 | `arguments.newappender → system/logging/AbstractAppender.cfc` | 1 | high | declares getName(), isInitialized(), onRegistration(), setInitialized() | `system/logging/Logger.cfc:150` variable 'arguments.newAppender' has no component ref |
| 3 | `arguments.target → system/aop/MixerUtil.cfc` | 1 | high | declares $wbAOPStoreJointPoint(), $wbAOPRemove(), $wbAOPInclude() | `system/aop/Mixer.cfc:383` variable 'arguments.target' has no component ref |
| 3 | `arguments.thismapping → system/ioc/config/Mapping.cfc` | 1 | high | declares isEagerInit(), getName() | `system/ioc/config/Binder.cfc:1344` variable 'arguments.thisMapping' has no component ref |
| 3 | `asyncmanager.$executors → system/async/executors/ExecutorBuilder.cfc` | 1 | high | declares newFixedThreadPool(), newCachedThreadPool() | `tests/specs/async/AsyncManagerSpec.cfc:25` variable 'asyncManager.$executors' has no component ref |
| 3 | `eventmanager → system/FrameworkSupertype.cfc` | 5 | low | declares announce() (5 candidates) | `system/ioc/config/Mapping.cfc:585` variable 'eventManager' has no component ref |
| 3 | `exceptionbean → system/web/context/ExceptionBean.cfc` | 2 | high | declares getMessage(), getExceptionStruct(); named like the receiver 'exceptionBean' | `test-harness/handlers/main.cfc:159` variable 'exceptionBean' has no component ref |
| 3 | `header.name` | 0 | none |  | `system/web/context/Response.cfc:232` variable 'header.name' has no component ref |
| 3 | `lastday` | 0 | none |  | `system/async/time/DateTimeHelper.cfc:272` variable 'lastDay' has no component ref |
| 3 | `mconfig.injector → system/ioc/config/Mapping.cfc` | 1 | medium | declares addDIConstructorArgument() | `system/web/services/ModuleService.cfc:823` variable 'mConfig.injector' has no component ref |
| 3 | `now → system/ioc/config/Mapping.cfc` | 2 | low | declares getValue() (2 candidates) | `system/async/tasks/ScheduledTask.cfc:594` variable 'now' has no component ref |
| 3 | `response → system/web/context/Response.cfc` | 1 | high | declares getDataPacket(), getError(), getStatusCode(); named like the receiver 'response' | `tests/specs/integration/SubscribersSpec.cfc:27` variable 'response' has no component ref |
| 3 | `targetobject` | 0 | none |  | `system/ioc/Injector.cfc:904` variable 'targetObject' has no component ref |
| 3 | `variables.executor → system/async/executors/ScheduledExecutor.cfc` | 1 | high | declares scheduleWithFixedDelay(), scheduleAtFixedRate(), schedule() | `system/async/tasks/ScheduledTask.cfc:861` variable 'variables.executor' has no component ref |
| 3 | `variables.native → system/async/executors/ScheduledExecutor.cfc` | 1 | medium | declares schedule() | `system/async/executors/ScheduledExecutor.cfc:57` variable 'variables.native' has no component ref |
| 3 | `variables.native → system/async/tasks/Future.cfc` | 2 | low | declares cancel() (2 candidates) | `system/async/tasks/FutureTask.cfc:38` variable 'variables.native' has no component ref |
| 2 | `application.cbcontroller → system/FrameworkSupertype.cfc` | 2 | low | declares setSetting() (2 candidates) | `tests/specs/remote/coldboxproxytest.cfc:42` variable 'application.cbController' has no component ref |
| 2 | `application[] → system/web/services/LoaderService.cfc` | 1 | high | declares loadApplication(); named like the receiver 'LoaderService' | `system/Bootstrap.cfc:101` variable 'application[]' has no component ref |
| 2 | `application[] → system/web/services/ModuleService.cfc` | 1 | high | declares loadMappings(); named like the receiver 'ModuleService' | `system/Bootstrap.cfc:163` variable 'application[]' has no component ref |
| 2 | `arguments.childinstance → system/ioc/IInjector.cfc` | 15 | low | declares shutdown() (15 candidates) | `system/ioc/Injector.cfc:435` variable 'arguments.childInstance' has no component ref |
| 2 | `arguments.future → system/async/tasks/Future.cfc` | 5 | medium | declares getNative(); named like the receiver 'future' (5 candidates) | `system/async/tasks/Future.cfc:754` variable 'arguments.future' has no component ref |
| 2 | `arguments.invocation → system/aop/MethodInvocation.cfc` | 1 | medium | declares proceed() | `tests/specs/ioc/aop/MethodInvocationTest.cfc:102` variable 'arguments.invocation' has no component ref |
| 2 | `arguments.logboxconfig → system/logging/config/LogBoxConfig.cfc` | 7 | medium | declares reset(), init(), getMemento(); named like the receiver 'logBoxConfig' (4 candidates) | `system/web/config/ApplicationLoader.cfc:844` variable 'arguments.logBoxConfig' has no component ref |
| 2 | `arguments.oconfig` | 1 | none |  | `system/web/config/ApplicationLoader.cfc:800` variable 'arguments.oConfig' has no component ref |
| 2 | `arguments.oeventhandler → system/EventHandler.cfc` | 1 | high | declares _actionExists(), _privateInvoker(); named like the receiver 'oEventHandler' | `system/web/services/HandlerService.cfc:950` variable 'arguments.oEventHandler' has no component ref |
| 2 | `arguments.schedule → system/async/executors/Executor.cfc` | 1 | high | declares shutdownNow(), shutdownAndAwaitTermination() | `system/async/AsyncManager.cfc:287` variable 'arguments.schedule' has no component ref |
| 2 | `arguments.scheduler → system/async/tasks/Scheduler.cfc` | 1 | high | declares getInetHost(), getLocalIp(); named like the receiver 'scheduler' | `system/async/tasks/ScheduledTask.cfc:270` variable 'arguments.scheduler' has no component ref |
| 2 | `arguments.targetobject` | 1 | none |  | `system/ioc/Injector.cfc:1225` variable 'arguments.targetObject' has no component ref |
| 2 | `arguments.thisitem` | 0 | none |  | `system/async/time/Duration.cfc:117` variable 'arguments.thisItem' has no component ref |
| 2 | `e → system/web/context/ExceptionBean.cfc` | 2 | high | declares getType(), getMessage() | `tests/specs/integration/EventExecutionsSpec.cfc:59` variable 'e' has no component ref |
| 2 | `interceptorentries` | 0 | none |  | `system/web/context/InterceptorState.cfc:490` variable 'interceptorEntries' has no component ref |
| 2 | `interceptorentry` | 0 | none |  | `system/web/context/InterceptorState.cfc:492` variable 'interceptorEntry' has no component ref |
| 2 | `local.results.ehbean → system/web/context/EventHandlerBean.cfc` | 1 | medium | declares getActionMetadata() | `system/web/Controller.cfc:744` variable 'local.results.ehBean' has no component ref |
| 2 | `nextrun → system/async/time/Period.cfc` | 1 | medium | declares plusMonths() | `system/async/tasks/ScheduledTask.cfc:1168` variable 'nextRun' has no component ref |
| 2 | `now → system/async/time/Duration.cfc` | 1 | medium | declares compareTo() | `system/async/tasks/ScheduledTask.cfc:1197` variable 'now' has no component ref |
| 2 | `oeventhandler → system/EventHandler.cfc` | 1 | high | declares _actionExists(); named like the receiver 'oEventHandler' | `system/web/services/HandlerService.cfc:148` variable 'oEventHandler' has no component ref |
| 2 | `ormfactory` | 0 | none |  | `tests/tmp/ormTest.cfm:21` variable 'ormFactory' has no component ref |
| 2 | `router → system/web/routing/Router.cfc` | 2 | high | declares route(), getRoutes(); named like the receiver 'router' | `tests/specs/web/routing/RouterSSETest.cfc:180` variable 'router' has no component ref |
| 2 | `services.requestservice → system/web/context/RequestContext.cfc` | 1 | high | declares getCurrentEvent(), renderdata() | `system/web/Controller.cfc:704` variable 'services.requestService' has no component ref |
| 2 | `services.schedulerservice → system/web/services/SchedulerService.cfc` | 1 | high | declares loadGlobalScheduler(), startupSchedulers(); named like the receiver 'schedulerService' | `system/web/services/LoaderService.cfc:101` variable 'services.schedulerService' has no component ref |
| 2 | `target → system/ioc/config/Binder.cfc` | 1 | medium | declares with() | `system/async/time/DateTimeHelper.cfc:313` variable 'target' has no component ref |
| 2 | `targetobject → system/core/dynamic/MixerUtil.cfc` | 1 | medium | declares injectMixin() | `system/ioc/Injector.cfc:1139` variable 'targetObject' has no component ref |
| 2 | `task → system/cache/AbstractCacheBoxProvider.cfc` | 8 | low | declares getStats() (8 candidates) | `test-harness/config/Scheduler.cfc:41` variable 'task' has no component ref |
| 2 | `taskrecord.task → system/async/tasks/ScheduledTask.cfc` | 1 | high | declares isDisabled(), getName(), start() | `system/async/tasks/Scheduler.cfc:185` variable 'taskRecord.task' has no component ref |
| 2 | `thismapping → system/ioc/config/Mapping.cfc` | 1 | medium | declares setVirtualInheritance() | `system/ioc/config/Binder.cfc:786` variable 'thisMapping' has no component ref |
| 2 | `variables.elementcleaner → system/cache/util/ElementCleaner.cfc` | 2 | medium | declares clearByKeySnippet(); named like the receiver 'elementCleaner' (2 candidates) | `system/cache/AbstractCacheBoxProvider.cfc:663` variable 'variables.elementCleaner' has no component ref |
| 2 | `variables.native` | 0 | none |  | `system/async/executors/Executor.cfc:419` variable 'variables.native' has no component ref |
| 2 | `variables.native → system/async/tasks/ScheduledFuture.cfc` | 1 | medium | declares isPeriodic() | `system/async/tasks/ScheduledFuture.cfc:28` variable 'variables.native' has no component ref |
| 2 | `variables.stats.lastresult` | 0 | none |  | `system/async/tasks/ScheduledTask.cfc:713` variable 'variables.stats.lastResult' has no component ref |
| 2 | `variables.system.err` | 1 | none |  | `system/async/AsyncManager.cfc:457` variable 'variables.System.err' has no component ref |
| 2 | `variables.system.out` | 1 | none |  | `system/async/AsyncManager.cfc:447` variable 'variables.System.out' has no component ref |
| 1 | `$parent → test-harness/models/delegates/Computer.cfc` | 1 | medium | declares getOutput() | `test-harness/models/delegates/Worker.cfc:21` variable '$parent' has no component ref |
| 1 | `_routes[]` | 0 | none |  | `system/web/services/RoutingService.cfc:589` variable '_routes[]' has no component ref |
| 1 | `anchor → system/async/time/Duration.cfc` | 1 | high | declares plusSeconds(), toString() | `system/async/tasks/ScheduledTask.cfc:1589` variable 'anchor' has no component ref |
| 1 | `application.cbbootstrap → system/remote/ColdboxProxy.cfc` | 2 | low | declares getCOLDBOX_APP_KEY() (2 candidates) | `system/remote/ColdboxProxy.cfc:32` variable 'application.cbBootstrap' has no component ref |
| 1 | `application.wirebox → system/aop/Mixer.cfc` | 2 | low | declares getBinder() (2 candidates) | `test-harness/index.cfm:11` variable 'application.wirebox' has no component ref |
| 1 | `application.wirebox → system/ioc/config/Binder.cfc` | 2 | medium | declares getMappings(); named like the receiver 'Binder' (2 candidates) | `test-harness/index.cfm:11` variable 'application.wirebox' has no component ref |
| 1 | `application[] → system/logging/LogBox.cfc` | 9 | medium | declares getLogger(); named like the receiver 'LogBox' (8 candidates) | `system/Bootstrap.cfc:131` variable 'application[]' has no component ref |
| 1 | `application[] → system/logging/Logger.cfc` | 2 | low | declares warn() (2 candidates) | `system/Bootstrap.cfc:539` variable 'application[]' has no component ref |
| 1 | `application[] → system/web/services/InterceptorService.cfc` | 5 | medium | declares announce(); named like the receiver 'InterceptorService' (5 candidates) | `system/Bootstrap.cfc:165` variable 'application[]' has no component ref |
| 1 | `arguments.appender → system/logging/AbstractAppender.cfc` | 15 | low | declares shutdown() (15 candidates) | `system/logging/LogBox.cfc:216` variable 'arguments.appender' has no component ref |
| 1 | `arguments.asyncmanager → system/async/AsyncManager.cfc` | 5 | medium | declares out(); named like the receiver 'asyncManager' (5 candidates) | `system/async/tasks/Scheduler.cfc:102` variable 'arguments.asyncManager' has no component ref |
| 1 | `arguments.child → system/ioc/IInjector.cfc` | 2 | low | declares setParent() (2 candidates) | `system/ioc/Injector.cfc:278` variable 'arguments.child' has no component ref |
| 1 | `arguments.data → test-harness/models/RenderConvention.cfc` | 1 | medium | declares $renderdata() | `system/core/conversion/DataMarshaller.cfc:60` variable 'arguments.data' has no component ref |
| 1 | `arguments.delegate → system/core/dynamic/MixerUtil.cfc` | 1 | medium | declares injectPropertyMixin() | `system/ioc/Injector.cfc:1415` variable 'arguments.delegate' has no component ref |
| 1 | `arguments.eventdictionary` | 0 | none |  | `system/cache/util/EventURLFacade.cfc:60` variable 'arguments.eventDictionary' has no component ref |
| 1 | `arguments.eventhandlerbean → system/web/context/EventHandlerBean.cfc` | 1 | high | declares getActionMetadata(); named like the receiver 'eventHandlerBean' | `test-harness/handlers/eventcachingSuffix.cfc:13` variable 'arguments.eventHandlerBean' has no component ref |
| 1 | `arguments.ocontext → system/web/context/ExceptionBean.cfc` | 17 | low | declares getMemento() (16 candidates) | `system/web/context/RequestContextDecorator.cfc:21` variable 'arguments.oContext' has no component ref |
| 1 | `arguments.parentinjector → system/ioc/Injector.cfc` | 1 | medium | declares registerChildInjector() | `system/web/services/ModuleService.cfc:1260` variable 'arguments.parentInjector' has no component ref |
| 1 | `arguments.prc.exception → system/web/context/ExceptionBean.cfc` | 1 | high | declares getExceptionStruct(); named like the receiver 'exception' | `system/RestHandler.cfc:193` variable 'arguments.prc.exception' has no component ref |
| 1 | `arguments.prc.response → system/web/context/RequestContext.cfc` | 2 | low | declares setStatusCode() (2 candidates) | `system/RestHandler.cfc:560` variable 'arguments.prc.response' has no component ref |
| 1 | `arguments.record.task → system/async/tasks/ScheduledTask.cfc` | 8 | low | declares getStats() (8 candidates) | `system/async/tasks/Scheduler.cfc:417` variable 'arguments.record.task' has no component ref |
| 1 | `arguments.result.value → system/aop/Matcher.cfc` | 17 | low | declares getMemento() (16 candidates) | `tests/specs/async/AsyncManagerSpec.cfc:373` variable 'arguments.result.value' has no component ref |
| 1 | `arguments.target → system/EventHandler.cfc` | 1 | medium | declares _privateInvoker() | `system/web/Controller.cfc:1268` variable 'arguments.target' has no component ref |
| 1 | `arguments.thisexecutor → system/async/executors/Executor.cfc` | 8 | low | declares getStats() (8 candidates) | `system/async/AsyncManager.cfc:309` variable 'arguments.thisExecutor' has no component ref |
| 1 | `arguments.thismapping → system/ioc/config/Binder.cfc` | 11 | low | declares process() (11 candidates) | `system/ioc/config/Binder.cfc:550` variable 'arguments.thisMapping' has no component ref |
| 1 | `arguments.value` | 0 | none |  | `system/web/context/RequestContext.cfc:1924` variable 'arguments.value' has no component ref |
| 1 | `arguments.value → system/async/time/DateTimeHelper.cfc` | 1 | medium | declares toInstant() | `system/web/context/RequestContext.cfc:1924` variable 'arguments.value' has no component ref |
| 1 | `aroute` | 0 | none |  | `system/web/services/RoutingService.cfc:923` variable 'aRoute' has no component ref |
| 1 | `aspectbindings[].classes → system/aop/Matcher.cfc` | 1 | medium | declares matchClass() | `system/aop/Mixer.cfc:156` variable 'aspectBindings[].classes' has no component ref |
| 1 | `cache → system/cache/AbstractCacheBoxProvider.cfc` | 15 | low | declares shutdown() (15 candidates) | `system/cache/CacheFactory.cfc:527` variable 'cache' has no component ref |
| 1 | `cacheprovider → system/cache/AbstractCacheBoxProvider.cfc` | 2 | low | declares getName(), isReportingEnabled() (2 candidates) | `system/cache/report/skins/default/CacheReport.cfm:81` variable 'cacheProvider' has no component ref |
| 1 | `cachesession` | 0 | none |  | `system/cache/providers/CFProvider.cfc:499` variable 'cacheSession' has no component ref |
| 1 | `childinjector → system/ioc/IInjector.cfc` | 8 | low | declares getInstance() (7 candidates) | `system/ioc/Injector.cfc:499` variable 'childInjector' has no component ref |
| 1 | `e → system/logging/LogEvent.cfc` | 2 | low | declares getMessage() (2 candidates) | `tests/specs/ioc/InjectorTest.cfc:120` variable 'e' has no component ref |
| 1 | `ehbean → system/web/context/EventHandlerBean.cfc` | 1 | medium | declares getFullEvent() | `system/web/services/HandlerService.cfc:464` variable 'ehBean' has no component ref |
| 1 | `executor → system/async/executors/Executor.cfc` | 15 | medium | declares shutdown(); named like the receiver 'executor' (15 candidates) | `tests/suites/async/performance-parallel-tests.cfm:162` variable 'executor' has no component ref |
| 1 | `expectation.actual → system/testing/mock/web/MockSSEEmitter.cfc` | 1 | medium | declares getSentEvents() | `system/testing/CustomMatchers.cfc:170` variable 'expectation.actual' has no component ref |
| 1 | `handler → system/EventHandler.cfc` | 1 | medium | declares _actionMetadata() | `system/web/services/HandlerService.cfc:1026` variable 'handler' has no component ref |
| 1 | `interceptionstate → system/web/context/InterceptorState.cfc` | 11 | low | declares process() (11 candidates) | `system/web/services/InterceptorService.cfc:226` variable 'interceptionState' has no component ref |
| 1 | `lastday → system/ioc/config/Mapping.cfc` | 2 | low | declares getValue() (2 candidates) | `system/async/time/DateTimeHelper.cfc:272` variable 'lastDay' has no component ref |
| 1 | `local.results.data → test-harness/models/RenderConvention.cfc` | 1 | medium | declares $renderdata() | `system/web/Controller.cfc:733` variable 'local.results.data' has no component ref |
| 1 | `logevent → system/logging/LogEvent.cfc` | 1 | high | declares getTimestamp(); named like the receiver 'logevent' | `tests/specs/logging/MockLayout.cfc:4` variable 'logevent' has no component ref |
| 1 | `mappings[] → tests/specs/core/collections/ScopeStorageTest.cfc` | 5 | low | declares getScope() (5 candidates) | `tests/specs/ioc/config/BinderTest.cfc:543` variable 'mappings[]' has no component ref |
| 1 | `mconfig.injector → system/web/routing/Router.cfc` | 2 | low | declares to() (2 candidates) | `system/web/services/ModuleService.cfc:755` variable 'mConfig.injector' has no component ref |
| 1 | `mixer.afterinstanceautowire` | 0 | none |  | `tests/specs/integration/WireBoxSpec.cfc:14` variable 'mixer.afterInstanceAutowire' has no component ref |
| 1 | `mixer.afterinstanceautowire → system/cache/store/ConcurrentStore.cfc` | 2 | low | declares getPool() (2 candidates) | `tests/specs/integration/WireBoxSpec.cfc:14` variable 'mixer.afterInstanceAutowire' has no component ref |
| 1 | `my.headdata` | 0 | none |  | `system/remote/RemotingUtil.cfc:41` variable 'my.headData' has no component ref |
| 1 | `my.method` | 0 | none |  | `system/remote/RemotingUtil.cfc:32` variable 'my.method' has no component ref |
| 1 | `nextrun` | 0 | none |  | `system/async/tasks/ScheduledTask.cfc:1127` variable 'nextRun' has no component ref |
| 1 | `oappender → system/logging/AbstractAppender.cfc` | 2 | low | declares onUnRegistration() (2 candidates) | `system/logging/Logger.cfc:179` variable 'oAppender' has no component ref |
| 1 | `oeventurlfacade → system/cache/util/EventURLFacade.cfc` | 1 | high | declares buildEventKey(); named like the receiver 'oEventURLFacade' | `system/web/services/RequestService.cfc:169` variable 'oEventURLFacade' has no component ref |
| 1 | `orm` | 0 | none |  | `tests/tmp/ormTest.cfm:10` variable 'orm' has no component ref |
| 1 | `providertest.coolpizza → system/ioc/IProvider.cfc` | 2 | low | declares $get() (2 candidates) | `tests/specs/ioc/InjectorCreationTest.cfc:118` variable 'providerTest.coolPizza' has no component ref |
| 1 | `routeresults.route → test-harness/handlers/rendering.cfc` | 1 | medium | declares redirect() | `system/web/services/RoutingService.cfc:779` variable 'routeResults.route' has no component ref |
| 1 | `scheduler → system/async/tasks/Scheduler.cfc` | 2 | medium | declares restart(); named like the receiver 'scheduler' (2 candidates) | `system/web/services/SchedulerService.cfc:227` variable 'scheduler' has no component ref |
| 1 | `scope → system/testing/BaseTestCase.cfc` | 4 | low | declares put() (4 candidates) | `tests/specs/ioc/scopes/SingletonTest.cfc:58` variable 'scope' has no component ref |
| 1 | `services.handlerservice → system/web/context/EventHandlerBean.cfc` | 1 | medium | declares setIsPrivate() | `system/web/Controller.cfc:821` variable 'services.handlerService' has no component ref |
| 1 | `services.loaderservice → system/web/services/LoaderService.cfc` | 1 | high | declares createDefaultLogBox(); named like the receiver 'loaderService' | `system/web/Controller.cfc:140` variable 'services.loaderService' has no component ref |
| 1 | `services.moduleservice → system/web/services/ModuleService.cfc` | 1 | high | declares activateAllModules(); named like the receiver 'moduleService' | `system/web/services/LoaderService.cfc:91` variable 'services.moduleService' has no component ref |
| 1 | `services.requestservice → system/web/flash/AbstractFlashScope.cfc` | 4 | low | declares saveFlash() (4 candidates) | `system/web/Controller.cfc:575` variable 'services.requestService' has no component ref |
| 1 | `stime → system/async/time/Duration.cfc` | 2 | low | declares plusDays() (2 candidates) | `system/async/tasks/ScheduledTask.cfc:1704` variable 'sTime' has no component ref |
| 1 | `target → system/core/dynamic/MixerUtil.cfc` | 1 | medium | declares injectMixin() | `system/ioc/Injector.cfc:1431` variable 'target' has no component ref |
| 1 | `taskrecord.future → system/async/tasks/Future.cfc` | 2 | medium | declares cancel(); named like the receiver 'future' (2 candidates) | `system/async/tasks/Scheduler.cfc:468` variable 'taskRecord.future' has no component ref |
| 1 | `this.$wbinjector → system/ioc/IInjector.cfc` | 8 | low | declares getInstance() (7 candidates) | `system/ioc/Builder.cfc:149` variable 'this.$wbInjector' has no component ref |
| 1 | `transientcache` | 0 | none |  | `system/ioc/Injector.cfc:1268` variable 'transientCache' has no component ref |
| 1 | `variables.appsettings.coldboxconfig → system/core/dynamic/MixerUtil.cfc` | 1 | medium | declares getPropertyMixin() | `system/web/services/ModuleService.cfc:69` variable 'variables.appSettings.coldBoxConfig' has no component ref |
| 1 | `variables.cache` | 0 | none |  | `system/cache/providers/MockProvider.cfc:169` variable 'variables.cache' has no component ref |
| 1 | `variables.cache → system/cache/AbstractCacheBoxProvider.cfc` | 14 | low | declares lookup() (14 candidates) | `system/web/flash/ColdboxCacheFlash.cfc:62` variable 'variables.cache' has no component ref |
| 1 | `variables.cacheprovider → system/cache/AbstractCacheBoxProvider.cfc` | 7 | low | declares lookupQuiet() (7 candidates) | `system/ioc/scopes/CacheBox.cfc:138` variable 'variables.cacheProvider' has no component ref |
| 1 | `variables.cacheprovider → system/cache/CacheFactory.cfc` | 14 | low | declares getColdBox() (14 candidates) | `system/cache/util/EventURLFacade.cfc:29` variable 'variables.cacheProvider' has no component ref |
| 1 | `variables.cacheprovider → system/cache/providers/BoxLangColdBoxProvider.cfc` | 6 | low | declares getEventCacheKeyPrefix() (6 candidates) | `system/cache/util/EventURLFacade.cfc:170` variable 'variables.cacheProvider' has no component ref |
| 1 | `variables.cacheprovider → system/web/Controller.cfc` | 3 | low | declares getRequestService() (3 candidates) | `system/cache/util/EventURLFacade.cfc:29` variable 'variables.cacheProvider' has no component ref |
| 1 | `variables.cacheprovider → system/web/context/RequestContext.cfc` | 1 | medium | declares getSesBaseUrl() | `system/cache/util/EventURLFacade.cfc:29` variable 'variables.cacheProvider' has no component ref |
| 1 | `variables.cacheprovider → system/web/services/RequestService.cfc` | 3 | medium | declares getContext(); named like the receiver 'RequestService' (3 candidates) | `system/cache/util/EventURLFacade.cfc:29` variable 'variables.cacheProvider' has no component ref |
| 1 | `variables.configsettings` | 0 | none |  | `system/web/Controller.cfc:1089` variable 'variables.configSettings' has no component ref |
| 1 | `variables.currentmapping → system/ioc/config/Mapping.cfc` | 1 | medium | declares setDelegates() | `system/ioc/config/Binder.cfc:927` variable 'variables.currentMapping' has no component ref |
| 1 | `variables.eventpoolcontainer → system/core/events/EventPool.cfc` | 11 | low | declares process() (11 candidates) | `system/core/events/EventPoolManager.cfc:72` variable 'variables.eventPoolContainer' has no component ref |
| 1 | `variables.executor` | 1 | none |  | `system/async/tasks/Scheduler.cfc:370` variable 'variables.executor' has no component ref |
| 1 | `variables.extrainfo → tests/specs/logging/ExtraInfo.cfc` | 2 | medium | declares $toString(); named like the receiver 'extraInfo' (2 candidates) | `system/logging/LogEvent.cfc:95` variable 'variables.extraInfo' has no component ref |
| 1 | `variables.scheduler → system/async/tasks/Scheduler.cfc` | 16 | medium | declares getUtil(); named like the receiver 'scheduler' (15 candidates) | `system/async/tasks/ScheduledTask.cfc:207` variable 'variables.scheduler' has no component ref |
| 1 | `variables.scopes[] → system/ioc/scopes/CFScopes.cfc` | 6 | low | declares getFromScope() (6 candidates) | `system/ioc/Injector.cfc:584` variable 'variables.scopes[]' has no component ref |
| 1 | `variables.stats → system/cache/AbstractCacheBoxProvider.cfc` | 10 | low | declares clearStatistics() (10 candidates) | `system/cache/AbstractCacheBoxProvider.cfc:155` variable 'variables.stats' has no component ref |
| 1 | `variables.target → system/aop/MixerUtil.cfc` | 1 | medium | declares $wbAOPInvokeProxy() | `system/aop/MethodInvocation.cfc:129` variable 'variables.target' has no component ref |
| 1 | `variables.templatecache → system/cache/providers/BoxLangColdBoxProvider.cfc` | 6 | low | declares getEventURLFacade() (6 candidates) | `system/web/services/RequestService.cfc:151` variable 'variables.templateCache' has no component ref |

<details><summary>Groups with several candidates</summary>

- `arguments.mapping → system/ioc/config/Mapping.cfc` — 18 finding(s), 2 candidate(s):
  - medium `system/ioc/config/Mapping.cfc` — declares getObjectMetadata(); named like the receiver 'mapping'
  - low `system/cache/store/indexers/JDBCMetadataIndexer.cfc` — declares getObjectMetadata()
- `cachestats → system/cache/util/CacheStats.cfc` — 12 finding(s), 5 candidate(s):
  - medium `system/cache/util/CacheStats.cfc` — declares getHits(), getMisses(), getGarbageCollections(), getEvictionCount(); named like the receiver 'cacheStats'
  - low `system/cache/util/IStats.cfc` — declares getHits(), getMisses(), getGarbageCollections(), getEvictionCount()
  - low `system/cache/providers/stats/BoxLangStats.cfc` — declares getHits(), getMisses(), getGarbageCollections(), getEvictionCount()
  - low `system/cache/providers/stats/CFStats.cfc` — declares getHits(), getMisses(), getGarbageCollections(), getEvictionCount()
  - low `system/cache/providers/stats/LuceeStats.cfc` — declares getHits(), getMisses(), getGarbageCollections(), getEvictionCount()
- `variables.asyncmanager → system/async/AsyncManager.cfc` — 12 finding(s), 5 candidate(s):
  - medium `system/async/AsyncManager.cfc` — declares out(); named like the receiver 'asyncManager'
  - low `system/async/tasks/ScheduledTask.cfc` — declares out()
  - low `system/async/executors/Executor.cfc` — declares out()
  - low `system/logging/AbstractAppender.cfc` — declares out()
  - low `system/web/tasks/ColdBoxScheduledTask.cfc` — declares out()
- `emitter → system/web/context/SSEEmitter.cfc` — 10 finding(s), 2 candidate(s):
  - low `system/web/context/SSEEmitter.cfc` — declares send(), isClosed()
  - low `system/testing/mock/web/MockSSEEmitter.cfc` — declares send(), isClosed()
- `scope → system/ioc/scopes/CFScopes.cfc` — 9 finding(s), 6 candidate(s):
  - low `system/ioc/scopes/CFScopes.cfc` — declares getFromScope()
  - low `system/ioc/scopes/CacheBox.cfc` — declares getFromScope()
  - low `system/ioc/scopes/IScope.cfc` — declares getFromScope()
  - low `system/ioc/scopes/NoScope.cfc` — declares getFromScope()
  - low `system/ioc/scopes/RequestScope.cfc` — declares getFromScope()
  - low `system/ioc/scopes/Singleton.cfc` — declares getFromScope()
- `services.interceptorservice → system/web/services/InterceptorService.cfc` — 7 finding(s), 5 candidate(s):
  - medium `system/web/services/InterceptorService.cfc` — declares announce(); named like the receiver 'interceptorService'
  - low `system/FrameworkSupertype.cfc` — declares announce()
  - low `system/remote/ColdboxProxy.cfc` — declares announce()
  - low `system/testing/BaseTestCase.cfc` — declares announce()
  - low `system/core/events/EventPoolManager.cfc` — declares announce()
- `variables.executor → system/async/executors/Executor.cfc` — 6 finding(s), 5 candidate(s):
  - medium `system/async/executors/Executor.cfc` — declares getNative(); named like the receiver 'executor'
  - low `system/async/tasks/Future.cfc` — declares getNative()
  - low `system/async/tasks/FutureTask.cfc` — declares getNative()
  - low `system/async/time/Duration.cfc` — declares getNative()
  - low `system/async/time/Period.cfc` — declares getNative()
- `expectation.actual → system/web/context/RequestContext.cfc` — 5 finding(s), 2 candidate(s):
  - low `system/web/context/RequestContext.cfc` — declares getStatusCode(), getMemento()
  - low `system/web/context/Response.cfc` — declares getStatusCode(), getMemento()
- `request.testbox → system/logging/Logger.cfc` — 5 finding(s), 3 candidate(s):
  - low `system/logging/Logger.cfc` — declares debug()
  - low `system/async/tasks/ScheduledTask.cfc` — declares debug()
  - low `system/logging/config/LogBoxConfig.cfc` — declares debug()
- `variables.emitter → system/web/context/SSEEmitter.cfc` — 5 finding(s), 2 candidate(s):
  - low `system/web/context/SSEEmitter.cfc` — declares isClosed()
  - low `system/testing/mock/web/MockSSEEmitter.cfc` — declares isClosed()
- `services[] → system/web/services/BaseService.cfc` — 4 finding(s), 2 candidate(s):
  - low `system/web/services/BaseService.cfc` — declares onConfigurationLoad(), afterAspectsLoad()
  - low `system/web/services/RequestService.cfc` — declares onConfigurationLoad(), afterAspectsLoad()
- `application[] → system/FrameworkSupertype.cfc` — 3 finding(s), 2 candidate(s):
  - low `system/FrameworkSupertype.cfc` — declares getSetting(), getLog()
  - low `system/web/Controller.cfc` — declares getSetting(), getLog()
- `arguments.logboxconfig → system/cache/config/CacheBoxConfig.cfc` — 3 finding(s), 2 candidate(s):
  - low `system/cache/config/CacheBoxConfig.cfc` — declares validate()
  - low `system/logging/config/LogBoxConfig.cfc` — declares validate()
- `eventmanager → system/FrameworkSupertype.cfc` — 3 finding(s), 5 candidate(s):
  - low `system/FrameworkSupertype.cfc` — declares announce()
  - low `system/remote/ColdboxProxy.cfc` — declares announce()
  - low `system/testing/BaseTestCase.cfc` — declares announce()
  - low `system/core/events/EventPoolManager.cfc` — declares announce()
  - low `system/web/services/InterceptorService.cfc` — declares announce()
- `now → system/ioc/config/Mapping.cfc` — 3 finding(s), 2 candidate(s):
  - low `system/ioc/config/Mapping.cfc` — declares getValue()
  - low `system/web/context/RequestContext.cfc` — declares getValue()
- `variables.native → system/async/tasks/Future.cfc` — 3 finding(s), 2 candidate(s):
  - low `system/async/tasks/Future.cfc` — declares cancel()
  - low `system/async/tasks/FutureTask.cfc` — declares cancel()
- `application.cbcontroller → system/FrameworkSupertype.cfc` — 2 finding(s), 2 candidate(s):
  - low `system/FrameworkSupertype.cfc` — declares setSetting()
  - low `system/web/Controller.cfc` — declares setSetting()
- `arguments.childinstance → system/ioc/IInjector.cfc` — 2 finding(s), 15 candidate(s):
  - low `system/ioc/IInjector.cfc` — declares shutdown()
  - low `system/ioc/Injector.cfc` — declares shutdown()
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares shutdown()
  - low `system/cache/CacheFactory.cfc` — declares shutdown()
  - low `system/logging/AbstractAppender.cfc` — declares shutdown()
  - low `system/logging/LogBox.cfc` — declares shutdown()
  - low `system/testing/VirtualApp.cfc` — declares shutdown()
  - low `system/async/executors/Executor.cfc` — declares shutdown()
  - … 7 more
- `arguments.future → system/async/tasks/Future.cfc` — 2 finding(s), 5 candidate(s):
  - medium `system/async/tasks/Future.cfc` — declares getNative(); named like the receiver 'future'
  - low `system/async/tasks/FutureTask.cfc` — declares getNative()
  - low `system/async/executors/Executor.cfc` — declares getNative()
  - low `system/async/time/Duration.cfc` — declares getNative()
  - low `system/async/time/Period.cfc` — declares getNative()
- `arguments.logboxconfig → system/logging/config/LogBoxConfig.cfc` — 2 finding(s), 4 candidate(s):
  - medium `system/logging/config/LogBoxConfig.cfc` — declares reset(), init(), getMemento(); named like the receiver 'logBoxConfig'
  - low `system/aop/Matcher.cfc` — declares reset(), init(), getMemento()
  - low `system/cache/config/CacheBoxConfig.cfc` — declares reset(), init(), getMemento()
  - low `system/ioc/config/Binder.cfc` — declares reset(), init(), getMemento()
- `task → system/cache/AbstractCacheBoxProvider.cfc` — 2 finding(s), 8 candidate(s):
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares getStats()
  - low `system/async/executors/Executor.cfc` — declares getStats()
  - low `system/async/tasks/ScheduledTask.cfc` — declares getStats()
  - low `system/cache/providers/BoxLangProvider.cfc` — declares getStats()
  - low `system/cache/providers/CFProvider.cfc` — declares getStats()
  - low `system/cache/providers/CacheBoxProvider.cfc` — declares getStats()
  - low `system/cache/providers/ICacheProvider.cfc` — declares getStats()
  - low `system/cache/providers/LuceeProvider.cfc` — declares getStats()
- `variables.elementcleaner → system/cache/util/ElementCleaner.cfc` — 2 finding(s), 2 candidate(s):
  - medium `system/cache/util/ElementCleaner.cfc` — declares clearByKeySnippet(); named like the receiver 'elementCleaner'
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares clearByKeySnippet()
- `application.cbbootstrap → system/remote/ColdboxProxy.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/remote/ColdboxProxy.cfc` — declares getCOLDBOX_APP_KEY()
  - low `system/Bootstrap.cfc` — declares getCOLDBOX_APP_KEY()
- `application.wirebox → system/aop/Mixer.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/aop/Mixer.cfc` — declares getBinder()
  - low `system/ioc/Injector.cfc` — declares getBinder()
- `application.wirebox → system/ioc/config/Binder.cfc` — 1 finding(s), 2 candidate(s):
  - medium `system/ioc/config/Binder.cfc` — declares getMappings(); named like the receiver 'Binder'
  - low `system/aop/Matcher.cfc` — declares getMappings()
- `application[] → system/logging/LogBox.cfc` — 1 finding(s), 8 candidate(s):
  - medium `system/logging/LogBox.cfc` — declares getLogger(); named like the receiver 'LogBox'
  - low `system/remote/ColdboxProxy.cfc` — declares getLogger()
  - low `system/cache/policies/AbstractEvictionPolicy.cfc` — declares getLogger()
  - low `system/web/context/InterceptorState.cfc` — declares getLogger()
  - low `system/web/services/BaseService.cfc` — declares getLogger()
  - low `system/web/services/ModuleService.cfc` — declares getLogger()
  - low `test-harness/models/testModel.cfc` — declares getLogger()
  - low `tests/specs/integration/MainSpec.cfc` — declares getLogger()
- `application[] → system/logging/Logger.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/logging/Logger.cfc` — declares warn()
  - low `system/logging/config/LogBoxConfig.cfc` — declares warn()
- `application[] → system/web/services/InterceptorService.cfc` — 1 finding(s), 5 candidate(s):
  - medium `system/web/services/InterceptorService.cfc` — declares announce(); named like the receiver 'InterceptorService'
  - low `system/FrameworkSupertype.cfc` — declares announce()
  - low `system/remote/ColdboxProxy.cfc` — declares announce()
  - low `system/testing/BaseTestCase.cfc` — declares announce()
  - low `system/core/events/EventPoolManager.cfc` — declares announce()
- `arguments.appender → system/logging/AbstractAppender.cfc` — 1 finding(s), 15 candidate(s):
  - low `system/logging/AbstractAppender.cfc` — declares shutdown()
  - low `system/logging/LogBox.cfc` — declares shutdown()
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares shutdown()
  - low `system/cache/CacheFactory.cfc` — declares shutdown()
  - low `system/ioc/IInjector.cfc` — declares shutdown()
  - low `system/ioc/Injector.cfc` — declares shutdown()
  - low `system/testing/VirtualApp.cfc` — declares shutdown()
  - low `system/async/executors/Executor.cfc` — declares shutdown()
  - … 7 more
- `arguments.asyncmanager → system/async/AsyncManager.cfc` — 1 finding(s), 5 candidate(s):
  - medium `system/async/AsyncManager.cfc` — declares out(); named like the receiver 'asyncManager'
  - low `system/async/tasks/ScheduledTask.cfc` — declares out()
  - low `system/async/executors/Executor.cfc` — declares out()
  - low `system/logging/AbstractAppender.cfc` — declares out()
  - low `system/web/tasks/ColdBoxScheduledTask.cfc` — declares out()
- `arguments.child → system/ioc/IInjector.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/ioc/IInjector.cfc` — declares setParent()
  - low `system/ioc/Injector.cfc` — declares setParent()
- `arguments.ocontext → system/web/context/ExceptionBean.cfc` — 1 finding(s), 16 candidate(s):
  - low `system/web/context/ExceptionBean.cfc` — declares getMemento()
  - low `system/web/context/RequestContext.cfc` — declares getMemento()
  - low `system/web/context/Response.cfc` — declares getMemento()
  - low `system/web/Controller.cfc` — declares getMemento()
  - low `system/web/routing/Router.cfc` — declares getMemento()
  - low `system/aop/Matcher.cfc` — declares getMemento()
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares getMemento()
  - low `system/async/tasks/ScheduledTask.cfc` — declares getMemento()
  - … 8 more
- `arguments.prc.response → system/web/context/RequestContext.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/web/context/RequestContext.cfc` — declares setStatusCode()
  - low `system/web/context/Response.cfc` — declares setStatusCode()
- `arguments.record.task → system/async/tasks/ScheduledTask.cfc` — 1 finding(s), 8 candidate(s):
  - low `system/async/tasks/ScheduledTask.cfc` — declares getStats()
  - low `system/async/executors/Executor.cfc` — declares getStats()
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares getStats()
  - low `system/cache/providers/BoxLangProvider.cfc` — declares getStats()
  - low `system/cache/providers/CFProvider.cfc` — declares getStats()
  - low `system/cache/providers/CacheBoxProvider.cfc` — declares getStats()
  - low `system/cache/providers/ICacheProvider.cfc` — declares getStats()
  - low `system/cache/providers/LuceeProvider.cfc` — declares getStats()
- `arguments.result.value → system/aop/Matcher.cfc` — 1 finding(s), 16 candidate(s):
  - low `system/aop/Matcher.cfc` — declares getMemento()
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares getMemento()
  - low `system/web/Controller.cfc` — declares getMemento()
  - low `system/async/tasks/ScheduledTask.cfc` — declares getMemento()
  - low `system/cache/config/CacheBoxConfig.cfc` — declares getMemento()
  - low `system/cache/util/CacheStats.cfc` — declares getMemento()
  - low `system/ioc/config/Binder.cfc` — declares getMemento()
  - low `system/ioc/config/Mapping.cfc` — declares getMemento()
  - … 8 more
- `arguments.thisexecutor → system/async/executors/Executor.cfc` — 1 finding(s), 8 candidate(s):
  - low `system/async/executors/Executor.cfc` — declares getStats()
  - low `system/async/tasks/ScheduledTask.cfc` — declares getStats()
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares getStats()
  - low `system/cache/providers/BoxLangProvider.cfc` — declares getStats()
  - low `system/cache/providers/CFProvider.cfc` — declares getStats()
  - low `system/cache/providers/CacheBoxProvider.cfc` — declares getStats()
  - low `system/cache/providers/ICacheProvider.cfc` — declares getStats()
  - low `system/cache/providers/LuceeProvider.cfc` — declares getStats()
- `arguments.thismapping → system/ioc/config/Binder.cfc` — 1 finding(s), 11 candidate(s):
  - low `system/ioc/config/Binder.cfc` — declares process()
  - low `system/ioc/config/Mapping.cfc` — declares process()
  - low `system/ioc/dsl/CacheBoxDSL.cfc` — declares process()
  - low `system/ioc/dsl/ColdBoxDSL.cfc` — declares process()
  - low `system/ioc/dsl/IDSLBuilder.cfc` — declares process()
  - low `system/ioc/dsl/LogBoxDSL.cfc` — declares process()
  - low `system/remote/ColdboxProxy.cfc` — declares process()
  - low `system/core/events/EventPool.cfc` — declares process()
  - … 3 more
- `cache → system/cache/AbstractCacheBoxProvider.cfc` — 1 finding(s), 15 candidate(s):
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares shutdown()
  - low `system/cache/CacheFactory.cfc` — declares shutdown()
  - low `system/cache/providers/BoxLangProvider.cfc` — declares shutdown()
  - low `system/cache/providers/CFProvider.cfc` — declares shutdown()
  - low `system/cache/providers/CacheBoxProvider.cfc` — declares shutdown()
  - low `system/cache/providers/ICacheProvider.cfc` — declares shutdown()
  - low `system/cache/providers/LuceeProvider.cfc` — declares shutdown()
  - low `system/cache/providers/MockProvider.cfc` — declares shutdown()
  - … 7 more
- `cacheprovider → system/cache/AbstractCacheBoxProvider.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares getName(), isReportingEnabled()
  - low `system/cache/providers/ICacheProvider.cfc` — declares getName(), isReportingEnabled()
- `childinjector → system/ioc/IInjector.cfc` — 1 finding(s), 7 candidate(s):
  - low `system/ioc/IInjector.cfc` — declares getInstance()
  - low `system/ioc/Injector.cfc` — declares getInstance()
  - low `system/FrameworkSupertype.cfc` — declares getInstance()
  - low `system/remote/ColdboxProxy.cfc` — declares getInstance()
  - low `system/testing/BaseTestCase.cfc` — declares getInstance()
  - low `test-harness/models/formBean.cfc` — declares getInstance()
  - low `test-harness/models/formImplicitBean.cfc` — declares getInstance()
- `e → system/logging/LogEvent.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/logging/LogEvent.cfc` — declares getMessage()
  - low `system/web/context/ExceptionBean.cfc` — declares getMessage()
- `executor → system/async/executors/Executor.cfc` — 1 finding(s), 15 candidate(s):
  - medium `system/async/executors/Executor.cfc` — declares shutdown(); named like the receiver 'executor'
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares shutdown()
  - low `system/cache/CacheFactory.cfc` — declares shutdown()
  - low `system/ioc/IInjector.cfc` — declares shutdown()
  - low `system/ioc/Injector.cfc` — declares shutdown()
  - low `system/logging/AbstractAppender.cfc` — declares shutdown()
  - low `system/logging/LogBox.cfc` — declares shutdown()
  - low `system/testing/VirtualApp.cfc` — declares shutdown()
  - … 7 more
- `interceptionstate → system/web/context/InterceptorState.cfc` — 1 finding(s), 11 candidate(s):
  - low `system/web/context/InterceptorState.cfc` — declares process()
  - low `system/remote/ColdboxProxy.cfc` — declares process()
  - low `system/core/events/EventPool.cfc` — declares process()
  - low `system/ioc/config/Binder.cfc` — declares process()
  - low `system/ioc/config/Mapping.cfc` — declares process()
  - low `system/ioc/dsl/CacheBoxDSL.cfc` — declares process()
  - low `system/ioc/dsl/ColdBoxDSL.cfc` — declares process()
  - low `system/ioc/dsl/IDSLBuilder.cfc` — declares process()
  - … 3 more
- `lastday → system/ioc/config/Mapping.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/ioc/config/Mapping.cfc` — declares getValue()
  - low `system/web/context/RequestContext.cfc` — declares getValue()
- `mappings[] → tests/specs/core/collections/ScopeStorageTest.cfc` — 1 finding(s), 5 candidate(s):
  - low `tests/specs/core/collections/ScopeStorageTest.cfc` — declares getScope()
  - low `system/ioc/Injector.cfc` — declares getScope()
  - low `system/core/collections/ScopeStorage.cfc` — declares getScope()
  - low `system/ioc/config/Mapping.cfc` — declares getScope()
  - low `system/web/flash/AbstractFlashScope.cfc` — declares getScope()
- `mconfig.injector → system/web/routing/Router.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/web/routing/Router.cfc` — declares to()
  - low `system/ioc/config/Binder.cfc` — declares to()
- `mixer.afterinstanceautowire → system/cache/store/ConcurrentStore.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/cache/store/ConcurrentStore.cfc` — declares getPool()
  - low `system/core/events/EventPool.cfc` — declares getPool()
- `oappender → system/logging/AbstractAppender.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/logging/AbstractAppender.cfc` — declares onUnRegistration()
  - low `system/logging/appenders/SocketAppender.cfc` — declares onUnRegistration()
- `providertest.coolpizza → system/ioc/IProvider.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/ioc/IProvider.cfc` — declares $get()
  - low `system/ioc/Provider.cfc` — declares $get()
- `scheduler → system/async/tasks/Scheduler.cfc` — 1 finding(s), 2 candidate(s):
  - medium `system/async/tasks/Scheduler.cfc` — declares restart(); named like the receiver 'scheduler'
  - low `system/testing/VirtualApp.cfc` — declares restart()
- `scope → system/testing/BaseTestCase.cfc` — 1 finding(s), 4 candidate(s):
  - low `system/testing/BaseTestCase.cfc` — declares put()
  - low `system/core/collections/ScopeStorage.cfc` — declares put()
  - low `system/web/flash/AbstractFlashScope.cfc` — declares put()
  - low `system/web/routing/Router.cfc` — declares put()
- `services.requestservice → system/web/flash/AbstractFlashScope.cfc` — 1 finding(s), 4 candidate(s):
  - low `system/web/flash/AbstractFlashScope.cfc` — declares saveFlash()
  - low `system/web/flash/ColdboxCacheFlash.cfc` — declares saveFlash()
  - low `system/web/flash/MockFlash.cfc` — declares saveFlash()
  - low `system/web/flash/SessionFlash.cfc` — declares saveFlash()
- `stime → system/async/time/Duration.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/async/time/Duration.cfc` — declares plusDays()
  - low `system/async/time/Period.cfc` — declares plusDays()
- `taskrecord.future → system/async/tasks/Future.cfc` — 1 finding(s), 2 candidate(s):
  - medium `system/async/tasks/Future.cfc` — declares cancel(); named like the receiver 'future'
  - low `system/async/tasks/FutureTask.cfc` — declares cancel()
- `this.$wbinjector → system/ioc/IInjector.cfc` — 1 finding(s), 7 candidate(s):
  - low `system/ioc/IInjector.cfc` — declares getInstance()
  - low `system/ioc/Injector.cfc` — declares getInstance()
  - low `system/FrameworkSupertype.cfc` — declares getInstance()
  - low `system/remote/ColdboxProxy.cfc` — declares getInstance()
  - low `system/testing/BaseTestCase.cfc` — declares getInstance()
  - low `test-harness/models/formBean.cfc` — declares getInstance()
  - low `test-harness/models/formImplicitBean.cfc` — declares getInstance()
- `variables.cache → system/cache/AbstractCacheBoxProvider.cfc` — 1 finding(s), 14 candidate(s):
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares lookup()
  - low `system/logging/LogLevels.cfc` — declares lookup()
  - low `system/cache/providers/BoxLangProvider.cfc` — declares lookup()
  - low `system/cache/providers/CFProvider.cfc` — declares lookup()
  - low `system/cache/providers/CacheBoxProvider.cfc` — declares lookup()
  - low `system/cache/providers/ICacheProvider.cfc` — declares lookup()
  - low `system/cache/providers/LuceeProvider.cfc` — declares lookup()
  - low `system/cache/providers/MockProvider.cfc` — declares lookup()
  - … 6 more
- `variables.cacheprovider → system/cache/AbstractCacheBoxProvider.cfc` — 1 finding(s), 7 candidate(s):
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares lookupQuiet()
  - low `system/cache/providers/BoxLangProvider.cfc` — declares lookupQuiet()
  - low `system/cache/providers/CFProvider.cfc` — declares lookupQuiet()
  - low `system/cache/providers/CacheBoxProvider.cfc` — declares lookupQuiet()
  - low `system/cache/providers/ICacheProvider.cfc` — declares lookupQuiet()
  - low `system/cache/providers/LuceeProvider.cfc` — declares lookupQuiet()
  - low `system/cache/providers/MockProvider.cfc` — declares lookupQuiet()
- `variables.cacheprovider → system/cache/CacheFactory.cfc` — 1 finding(s), 14 candidate(s):
  - low `system/cache/CacheFactory.cfc` — declares getColdBox()
  - low `system/cache/providers/BoxLangColdBoxProvider.cfc` — declares getColdBox()
  - low `system/cache/providers/CFColdBoxProvider.cfc` — declares getColdBox()
  - low `system/cache/providers/CacheBoxColdBoxProvider.cfc` — declares getColdBox()
  - low `system/cache/providers/IColdBoxProvider.cfc` — declares getColdBox()
  - low `system/cache/providers/LuceeColdboxProvider.cfc` — declares getColdBox()
  - low `system/cache/providers/MockProvider.cfc` — declares getColdBox()
  - low `system/async/AsyncManager.cfc` — declares getColdBox()
  - … 6 more
- `variables.cacheprovider → system/cache/providers/BoxLangColdBoxProvider.cfc` — 1 finding(s), 6 candidate(s):
  - low `system/cache/providers/BoxLangColdBoxProvider.cfc` — declares getEventCacheKeyPrefix()
  - low `system/cache/providers/CFColdBoxProvider.cfc` — declares getEventCacheKeyPrefix()
  - low `system/cache/providers/CacheBoxColdBoxProvider.cfc` — declares getEventCacheKeyPrefix()
  - low `system/cache/providers/IColdBoxProvider.cfc` — declares getEventCacheKeyPrefix()
  - low `system/cache/providers/LuceeColdboxProvider.cfc` — declares getEventCacheKeyPrefix()
  - low `system/cache/providers/MockProvider.cfc` — declares getEventCacheKeyPrefix()
- `variables.cacheprovider → system/web/Controller.cfc` — 1 finding(s), 3 candidate(s):
  - low `system/web/Controller.cfc` — declares getRequestService()
  - low `system/core/conversion/DataMarshaller.cfc` — declares getRequestService()
  - low `system/modules/HTMLHelper/models/HTMLHelper.cfc` — declares getRequestService()
- `variables.cacheprovider → system/web/services/RequestService.cfc` — 1 finding(s), 3 candidate(s):
  - medium `system/web/services/RequestService.cfc` — declares getContext(); named like the receiver 'RequestService'
  - low `system/web/context/RequestContext.cfc` — declares getContext()
  - low `system/web/delegates/Routable.cfc` — declares getContext()
- `variables.eventpoolcontainer → system/core/events/EventPool.cfc` — 1 finding(s), 11 candidate(s):
  - low `system/core/events/EventPool.cfc` — declares process()
  - low `system/remote/ColdboxProxy.cfc` — declares process()
  - low `system/ioc/config/Binder.cfc` — declares process()
  - low `system/ioc/config/Mapping.cfc` — declares process()
  - low `system/ioc/dsl/CacheBoxDSL.cfc` — declares process()
  - low `system/ioc/dsl/ColdBoxDSL.cfc` — declares process()
  - low `system/ioc/dsl/IDSLBuilder.cfc` — declares process()
  - low `system/ioc/dsl/LogBoxDSL.cfc` — declares process()
  - … 3 more
- `variables.extrainfo → tests/specs/logging/ExtraInfo.cfc` — 1 finding(s), 2 candidate(s):
  - medium `tests/specs/logging/ExtraInfo.cfc` — declares $toString(); named like the receiver 'extraInfo'
  - low `system/web/context/ExceptionBean.cfc` — declares $toString()
- `variables.scheduler → system/async/tasks/Scheduler.cfc` — 1 finding(s), 15 candidate(s):
  - medium `system/async/tasks/Scheduler.cfc` — declares getUtil(); named like the receiver 'scheduler'
  - low `system/logging/AbstractAppender.cfc` — declares getUtil()
  - low `system/logging/LogEvent.cfc` — declares getUtil()
  - low `system/remote/ColdboxProxy.cfc` — declares getUtil()
  - low `system/testing/BaseTestCase.cfc` — declares getUtil()
  - low `system/web/Controller.cfc` — declares getUtil()
  - low `system/cache/policies/AbstractEvictionPolicy.cfc` — declares getUtil()
  - low `system/cache/report/ReportHandler.cfc` — declares getUtil()
  - … 7 more
- `variables.scopes[] → system/ioc/scopes/CFScopes.cfc` — 1 finding(s), 6 candidate(s):
  - low `system/ioc/scopes/CFScopes.cfc` — declares getFromScope()
  - low `system/ioc/scopes/CacheBox.cfc` — declares getFromScope()
  - low `system/ioc/scopes/IScope.cfc` — declares getFromScope()
  - low `system/ioc/scopes/NoScope.cfc` — declares getFromScope()
  - low `system/ioc/scopes/RequestScope.cfc` — declares getFromScope()
  - low `system/ioc/scopes/Singleton.cfc` — declares getFromScope()
- `variables.stats → system/cache/AbstractCacheBoxProvider.cfc` — 1 finding(s), 10 candidate(s):
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares clearStatistics()
  - low `system/cache/providers/BoxLangProvider.cfc` — declares clearStatistics()
  - low `system/cache/providers/CFProvider.cfc` — declares clearStatistics()
  - low `system/cache/providers/ICacheProvider.cfc` — declares clearStatistics()
  - low `system/cache/providers/LuceeProvider.cfc` — declares clearStatistics()
  - low `system/cache/util/CacheStats.cfc` — declares clearStatistics()
  - low `system/cache/util/IStats.cfc` — declares clearStatistics()
  - low `system/cache/providers/stats/BoxLangStats.cfc` — declares clearStatistics()
  - … 2 more
- `variables.templatecache → system/cache/providers/BoxLangColdBoxProvider.cfc` — 1 finding(s), 6 candidate(s):
  - low `system/cache/providers/BoxLangColdBoxProvider.cfc` — declares getEventURLFacade()
  - low `system/cache/providers/CFColdBoxProvider.cfc` — declares getEventURLFacade()
  - low `system/cache/providers/CacheBoxColdBoxProvider.cfc` — declares getEventURLFacade()
  - low `system/cache/providers/IColdBoxProvider.cfc` — declares getEventURLFacade()
  - low `system/cache/providers/LuceeColdboxProvider.cfc` — declares getEventURLFacade()
  - low `system/cache/providers/MockProvider.cfc` — declares getEventURLFacade()

</details>

## Return types — a call chained on a method that declares no component

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 22 | `method 'registerNewInstance' in coldbox.system.ioc.Injector has no component return type → system/ioc/config/Mapping.cfc` | 1 | medium | declares setCacheProperties() | `system/web/services/HandlerService.cfc:111` method 'registerNewInstance' in coldbox.system.ioc.Injector has no component return type (chain to 'setCacheProperties') |
| 17 | `method 'getCacheProvider' has no component return type → system/cache/providers/BoxLangProvider.cfc` | 12 | low | declares getCache() (12 candidates) | `system/cache/providers/stats/BoxLangStats.cfc:29` method 'getCacheProvider' has no component return type (chain to 'getCache') |
| 14 | `method 'getEventManager' has no component return type → system/FrameworkSupertype.cfc` | 5 | low | declares announce() (5 candidates) | `system/cache/providers/BoxLangProvider.cfc:370` method 'getEventManager' has no component return type (chain to 'announce') |
| 8 | `method 'getCacheProvider' has no component return type` | 0 | none |  | `system/cache/providers/stats/BoxLangStats.cfc:29` method 'getCacheProvider' has no component return type (chain to 'hitRate') |
| 6 | `method 'getScheduler' has no component return type → system/async/tasks/Scheduler.cfc` | 3 | low | declares beforeAnyTask(), afterAnyTask(), onAnyTaskSuccess(), onAnyTaskError(); named like the receiver 'Scheduler' (3 candidates) | `system/async/tasks/ScheduledTask.cfc:705` method 'getScheduler' has no component return type (chain to 'beforeAnyTask') |
| 6 | `method 'toLocalDateTime' has no component return type` | 0 | none |  | `system/async/time/DateTimeHelper.cfc:230` method 'toLocalDateTime' has no component return type (chain to 'withHour') |
| 5 | `method 'getJavaNow' in ScheduledTask has no component return type` | 0 | none |  | `tests/specs/async/tasks/ScheduledTaskSpec.cfc:329` method 'getJavaNow' in ScheduledTask has no component return type (chain to 'getDayOfMonth') |
| 5 | `method 'getQueue' has no component return type` | 0 | none |  | `system/async/executors/Executor.cfc:431` method 'getQueue' has no component return type (chain to 'remainingCapacity') |
| 4 | `method 'getExecutor' in coldbox.system.async.AsyncManager has no component return type → system/async/executors/Executor.cfc` | 8 | medium | declares getStats(); named like the receiver 'Executor' (8 candidates) | `tests/specs/async/tasks/ScheduledExecutorSpec.cfc:85` method 'getExecutor' in coldbox.system.async.AsyncManager has no component return type (chain to 'getStats') |
| 4 | `method 'getInetAddress' has no component return type` | 0 | none |  | `system/core/util/Util.cfc:101` method 'getInetAddress' has no component return type (chain to 'getHostName') |
| 4 | `method 'getTaskScheduler' in coldbox.system.cache.CacheFactory has no component return type → system/async/tasks/ScheduledTask.cfc` | 1 | medium | declares delay() | `system/cache/providers/CacheBoxProvider.cfc:236` method 'getTaskScheduler' in coldbox.system.cache.CacheFactory has no component return type (chain to 'delay') |
| 4 | `method 'getTimezone' has no component return type → tests/tmp/User.cfc` | 2 | low | declares getId() (2 candidates) | `system/web/tasks/ColdBoxScheduledTask.cfc:329` method 'getTimezone' has no component return type (chain to 'getId') |
| 3 | `method 'getCacheProvider' has no component return type → system/cache/providers/ICacheProvider.cfc` | 2 | low | declares getConfiguration() (2 candidates) | `system/cache/providers/stats/LuceeStats.cfc:45` method 'getCacheProvider' has no component return type (chain to 'getConfiguration') |
| 3 | `method 'getCacheStats' has no component return type` | 0 | none |  | `system/cache/providers/stats/CFStats.cfc:67` method 'getCacheStats' has no component return type (chain to 'cacheEvictionCount') |
| 3 | `method 'getColdBoxVirtualApp' has no component return type → system/testing/VirtualApp.cfc` | 4 | low | declares startup() (4 candidates) | `system/testing/BaseTestCase.cfc:115` method 'getColdBoxVirtualApp' has no component return type (chain to 'startup') |
| 3 | `method 'getJavaSystem' has no component return type → system/Interceptor.cfc` | 5 | high | declares getProperty(), getEnv() | `system/core/delegates/Env.cfc:15` method 'getJavaSystem' has no component return type (chain to 'getProperty') |
| 3 | `method 'getTaskScheduler' in coldbox.system.logging.LogBox has no component return type → system/async/tasks/ScheduledTask.cfc` | 1 | medium | declares delay() | `system/logging/appenders/RollingFileAppender.cfc:79` method 'getTaskScheduler' in coldbox.system.logging.LogBox has no component return type (chain to 'delay') |
| 2 | `method 'get' in coldbox.system.async.time.TimeUnit has no component return type → system/async/time/Duration.cfc` | 1 | medium | declares toSeconds() | `system/async/tasks/ScheduledTask.cfc:1557` method 'get' in coldbox.system.async.time.TimeUnit has no component return type (chain to 'toSeconds') |
| 2 | `method 'getInterceptors' has no component return type` | 0 | none |  | `system/web/context/InterceptorState.cfc:488` method 'getInterceptors' has no component return type (chain to 'entrySet') |
| 2 | `method 'getTaskScheduler' in coldbox.system.logging.LogBox has no component return type → system/async/executors/ScheduledExecutor.cfc` | 1 | medium | declares schedule() | `system/logging/AbstractAppender.cfc:319` method 'getTaskScheduler' in coldbox.system.logging.LogBox has no component return type (chain to 'schedule') |
| 2 | `method 'toInstant' has no component return type` | 0 | none |  | `system/async/time/DateTimeHelper.cfc:93` method 'toInstant' has no component return type (chain to 'atZone') |
| 2 | `method 'toInstant' has no component return type → system/async/time/DateTimeHelper.cfc` | 1 | medium | declares toLocalDateTime() | `system/async/time/DateTimeHelper.cfc:93` method 'toInstant' has no component return type (chain to 'toLocalDateTime') |
| 2 | `method 'toLocalDateTime' has no component return type → system/ioc/config/Binder.cfc` | 1 | medium | declares with() | `system/async/time/DateTimeHelper.cfc:230` method 'toLocalDateTime' has no component return type (chain to 'with') |
| 1 | `method 'createMock' in testbox.system.MockBox has no component return type` | 1 | none |  | `tests/specs/ioc/aop/MixerTest.cfc:19` method 'createMock' in testbox.system.MockBox has no component return type (chain to '$') |
| 1 | `method 'get' has no component return type` | 0 | none |  | `system/web/context/InterceptorBuffer.cfc:31` method 'get' has no component return type (chain to 'setLength') |
| 1 | `method 'getCacheBoxDSL' has no component return type → system/ioc/dsl/CacheBoxDSL.cfc` | 11 | medium | declares process(); named like the receiver 'CacheBoxDSL' (11 candidates) | `system/ioc/Builder.cfc:611` method 'getCacheBoxDSL' has no component return type (chain to 'process') |
| 1 | `method 'getCacheProvider' has no component return type → system/cache/providers/stats/BoxLangStats.cfc` | 10 | low | declares clearStatistics() (10 candidates) | `system/cache/providers/stats/BoxLangStats.cfc:48` method 'getCacheProvider' has no component return type (chain to 'clearStatistics') |
| 1 | `method 'getCacheStats' has no component return type → system/cache/providers/BoxLangProvider.cfc` | 14 | low | declares getSize() (14 candidates) | `system/cache/providers/stats/CFStats.cfc:44` method 'getCacheStats' has no component return type (chain to 'getSize') |
| 1 | `method 'getClassMappingHelper' has no component return type → system/core/util/BoxLangMappingHelper.cfc` | 4 | low | declares addCustomTagPath() (4 candidates) | `system/core/util/Util.cfc:30` method 'getClassMappingHelper' has no component return type (chain to 'addCustomTagPath') |
| 1 | `method 'getColdBoxDSL' has no component return type → system/ioc/dsl/ColdBoxDSL.cfc` | 11 | medium | declares process(); named like the receiver 'ColdBoxDSL' (11 candidates) | `system/ioc/Builder.cfc:630` method 'getColdBoxDSL' has no component return type (chain to 'process') |
| 1 | `method 'getColdBoxVirtualApp' has no component return type → system/cache/AbstractCacheBoxProvider.cfc` | 15 | low | declares shutdown() (15 candidates) | `tests/resources/BaseIntegrationTest.cfc:92` method 'getColdBoxVirtualApp' has no component return type (chain to 'shutdown') |
| 1 | `method 'getConfig' in coldbox.system.logging.LogBox has no component return type → system/logging/config/LogBoxConfig.cfc` | 1 | medium | declares getAllAppenders() | `tests/specs/cache/CacheBoxStandaloneSpec.cfc:36` method 'getConfig' in coldbox.system.logging.LogBox has no component return type (chain to 'getAllAppenders') |
| 1 | `method 'getConfig' in coldbox.system.logging.LogBox has no component return type → test-harness/models/lib/Config.cfc` | 17 | medium | declares getMemento(); named like the receiver 'Config' (16 candidates) | `system/web/config/ApplicationLoader.cfc:39` method 'getConfig' in coldbox.system.logging.LogBox has no component return type (chain to 'getMemento') |
| 1 | `method 'getExecutor' has no component return type → system/async/executors/Executor.cfc` | 8 | medium | declares getStats(); named like the receiver 'Executor' (8 candidates) | `system/async/AsyncManager.cfc:305` method 'getExecutor' has no component return type (chain to 'getStats') |
| 1 | `method 'getFirstBusinessDayOfTheMonth' in coldbox.system.async.time.DateTimeHelper has no component return type` | 0 | none |  | `system/async/tasks/ScheduledTask.cfc:602` method 'getFirstBusinessDayOfTheMonth' in coldbox.system.async.time.DateTimeHelper has no component return type (chain to 'getDayOfMonth') |
| 1 | `method 'getINterceptors' in coldbox.system.web.context.InterceptorState has no component return type → system/web/flash/AbstractFlashScope.cfc` | 1 | medium | declares size() | `tests/specs/web/context/InterceptorStateTest.cfc:104` method 'getINterceptors' in coldbox.system.web.context.InterceptorState has no component return type (chain to 'size') |
| 1 | `method 'getInstance' has no component return type → system/ioc/IProvider.cfc` | 2 | low | declares $get() (2 candidates) | `system/ioc/Injector.cfc:646` method 'getInstance' has no component return type (chain to '$get') |
| 1 | `method 'getInstance' in coldbox.system.ioc.Injector has no component return type → system/cache/AbstractCacheBoxProvider.cfc` | 15 | low | declares setName() (15 candidates) | `system/web/services/SchedulerService.cfc:113` method 'getInstance' in coldbox.system.ioc.Injector has no component return type (chain to 'setName') |
| 1 | `method 'getInstance' in coldbox.system.ioc.Injector has no component return type → system/web/routing/Router.cfc` | 1 | medium | declares findRouteByName() | `system/web/Controller.cfc:628` method 'getInstance' in coldbox.system.ioc.Injector has no component return type (chain to 'findRouteByName') |
| 1 | `method 'getInstance' in coldbox.system.ioc.Injector has no component return type → tests/resources/routing/SampleMiddleware.cfc` | 1 | medium | declares getWasCalled() | `tests/specs/web/routing/RoutingServiceTest.cfc:491` method 'getInstance' in coldbox.system.ioc.Injector has no component return type (chain to 'getWasCalled') |
| 1 | `method 'getInterceptors' in coldbox.system.web.context.InterceptorState has no component return type → system/web/flash/AbstractFlashScope.cfc` | 1 | medium | declares size() | `tests/specs/web/context/InterceptorStateTest.cfc:39` method 'getInterceptors' in coldbox.system.web.context.InterceptorState has no component return type (chain to 'size') |
| 1 | `method 'getJavaCollections' has no component return type → test-harness/models/PhotosService.cfc` | 3 | low | declares list() (3 candidates) | `system/cache/store/ConcurrentStore.cfc:69` method 'getJavaCollections' has no component return type (chain to 'list') |
| 1 | `method 'getJavaNow' in ScheduledTask has no component return type → system/ioc/config/Mapping.cfc` | 2 | low | declares getValue() (2 candidates) | `tests/specs/async/tasks/ScheduledTaskSpec.cfc:394` method 'getJavaNow' in ScheduledTask has no component return type (chain to 'getValue') |
| 1 | `method 'getJavaSystem' has no component return type → system/core/delegates/Env.cfc` | 6 | low | declares getEnv() (3 candidates) | `system/core/delegates/Env.cfc:68` method 'getJavaSystem' has no component return type (chain to 'getEnv') |
| 1 | `method 'getLastBusinessDayOfTheMonth' in coldbox.system.async.time.DateTimeHelper has no component return type` | 0 | none |  | `system/async/tasks/ScheduledTask.cfc:612` method 'getLastBusinessDayOfTheMonth' in coldbox.system.async.time.DateTimeHelper has no component return type (chain to 'getDayOfMonth') |
| 1 | `method 'getLastResult' in ScheduledTask has no component return type` | 0 | none |  | `tests/specs/async/tasks/ScheduledTaskSpec.cfc:51` method 'getLastResult' in ScheduledTask has no component return type (chain to 'isPresent') |
| 1 | `method 'getLogBoxDSL' has no component return type → system/ioc/dsl/LogBoxDSL.cfc` | 11 | medium | declares process(); named like the receiver 'LogBoxDSL' (11 candidates) | `system/ioc/Builder.cfc:655` method 'getLogBoxDSL' has no component return type (chain to 'process') |
| 1 | `method 'getScope' has no component return type → system/ioc/scopes/Singleton.cfc` | 1 | medium | declares clearAppOnly() | `system/ioc/Injector.cfc:1081` method 'getScope' has no component return type (chain to 'clearAppOnly') |
| 1 | `method 'getSystemTimezone' has no component return type → tests/tmp/User.cfc` | 2 | low | declares getId() (2 candidates) | `system/async/time/DateTimeHelper.cfc:168` method 'getSystemTimezone' has no component return type (chain to 'getId') |
| 1 | `method 'getTaskScheduler' in coldbox.system.cache.CacheFactory has no component return type → system/async/executors/ScheduledExecutor.cfc` | 1 | medium | declares newTask() | `system/cache/providers/CacheBoxProvider.cfc:236` method 'getTaskScheduler' in coldbox.system.cache.CacheFactory has no component return type (chain to 'newTask') |
| 1 | `method 'getUtility' has no component return type → system/cache/AbstractCacheBoxProvider.cfc` | 2 | low | declares inThread() (2 candidates) | `system/cache/AbstractCacheBoxProvider.cfc:744` method 'getUtility' has no component return type (chain to 'inThread') |
| 1 | `method 'getXmlConverter' has no component return type → system/core/conversion/XMLConverter.cfc` | 1 | high | declares toXML(); named like the receiver 'XmlConverter' | `system/logging/LogEvent.cfc:121` method 'getXmlConverter' has no component return type (chain to 'toXML') |
| 1 | `method 'now' in coldbox.system.async.time.DateTimeHelper has no component return type` | 0 | none |  | `system/web/tasks/ColdBoxScheduledTask.cfc:468` method 'now' in coldbox.system.async.time.DateTimeHelper has no component return type (chain to 'until') |
| 1 | `method 'unless' has no component return type → system/FrameworkSupertype.cfc` | 5 | low | declares when() (4 candidates) | `test-harness/handlers/main.cfc:59` method 'unless' has no component return type (chain to 'when') |

<details><summary>Groups with several candidates</summary>

- `method 'getCacheProvider' has no component return type → system/cache/providers/BoxLangProvider.cfc` — 17 finding(s), 12 candidate(s):
  - low `system/cache/providers/BoxLangProvider.cfc` — declares getCache()
  - low `system/cache/providers/MockProvider.cfc` — declares getCache()
  - low `system/cache/CacheFactory.cfc` — declares getCache()
  - low `system/FrameworkSupertype.cfc` — declares getCache()
  - low `system/cache/config/CacheBoxConfig.cfc` — declares getCache()
  - low `system/remote/ColdboxProxy.cfc` — declares getCache()
  - low `system/testing/BaseTestCase.cfc` — declares getCache()
  - low `system/web/Controller.cfc` — declares getCache()
  - … 4 more
- `method 'getEventManager' has no component return type → system/FrameworkSupertype.cfc` — 14 finding(s), 5 candidate(s):
  - low `system/FrameworkSupertype.cfc` — declares announce()
  - low `system/remote/ColdboxProxy.cfc` — declares announce()
  - low `system/testing/BaseTestCase.cfc` — declares announce()
  - low `system/core/events/EventPoolManager.cfc` — declares announce()
  - low `system/web/services/InterceptorService.cfc` — declares announce()
- `method 'getScheduler' has no component return type → system/async/tasks/Scheduler.cfc` — 6 finding(s), 3 candidate(s):
  - low `system/async/tasks/Scheduler.cfc` — declares beforeAnyTask(), afterAnyTask(), onAnyTaskSuccess(), onAnyTaskError(); named like the receiver 'Scheduler'
  - low `test-harness/config/Scheduler.cfc` — declares beforeAnyTask(), afterAnyTask(), onAnyTaskSuccess(), onAnyTaskError(); named like the receiver 'Scheduler'
  - low `tests/suites/loadtests/beload/config/Scheduler.cfc` — declares beforeAnyTask(), afterAnyTask(), onAnyTaskSuccess(), onAnyTaskError(); named like the receiver 'Scheduler'
- `method 'getExecutor' in coldbox.system.async.AsyncManager has no component return type → system/async/executors/Executor.cfc` — 4 finding(s), 8 candidate(s):
  - medium `system/async/executors/Executor.cfc` — declares getStats(); named like the receiver 'Executor'
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares getStats()
  - low `system/async/tasks/ScheduledTask.cfc` — declares getStats()
  - low `system/cache/providers/BoxLangProvider.cfc` — declares getStats()
  - low `system/cache/providers/CFProvider.cfc` — declares getStats()
  - low `system/cache/providers/CacheBoxProvider.cfc` — declares getStats()
  - low `system/cache/providers/ICacheProvider.cfc` — declares getStats()
  - low `system/cache/providers/LuceeProvider.cfc` — declares getStats()
- `method 'getTimezone' has no component return type → tests/tmp/User.cfc` — 4 finding(s), 2 candidate(s):
  - low `tests/tmp/User.cfc` — declares getId()
  - low `test-harness/models/entities/User.cfc` — declares getId()
- `method 'getCacheProvider' has no component return type → system/cache/providers/ICacheProvider.cfc` — 3 finding(s), 2 candidate(s):
  - low `system/cache/providers/ICacheProvider.cfc` — declares getConfiguration()
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares getConfiguration()
- `method 'getColdBoxVirtualApp' has no component return type → system/testing/VirtualApp.cfc` — 3 finding(s), 4 candidate(s):
  - low `system/testing/VirtualApp.cfc` — declares startup()
  - low `system/web/Renderer.cfc` — declares startup()
  - low `system/async/tasks/Scheduler.cfc` — declares startup()
  - low `system/web/routing/Router.cfc` — declares startup()
- `method 'getCacheBoxDSL' has no component return type → system/ioc/dsl/CacheBoxDSL.cfc` — 1 finding(s), 11 candidate(s):
  - medium `system/ioc/dsl/CacheBoxDSL.cfc` — declares process(); named like the receiver 'CacheBoxDSL'
  - low `system/ioc/config/Binder.cfc` — declares process()
  - low `system/ioc/config/Mapping.cfc` — declares process()
  - low `system/ioc/dsl/ColdBoxDSL.cfc` — declares process()
  - low `system/ioc/dsl/IDSLBuilder.cfc` — declares process()
  - low `system/ioc/dsl/LogBoxDSL.cfc` — declares process()
  - low `system/remote/ColdboxProxy.cfc` — declares process()
  - low `system/core/events/EventPool.cfc` — declares process()
  - … 3 more
- `method 'getCacheProvider' has no component return type → system/cache/providers/stats/BoxLangStats.cfc` — 1 finding(s), 10 candidate(s):
  - low `system/cache/providers/stats/BoxLangStats.cfc` — declares clearStatistics()
  - low `system/cache/providers/stats/CFStats.cfc` — declares clearStatistics()
  - low `system/cache/providers/stats/LuceeStats.cfc` — declares clearStatistics()
  - low `system/cache/providers/BoxLangProvider.cfc` — declares clearStatistics()
  - low `system/cache/providers/CFProvider.cfc` — declares clearStatistics()
  - low `system/cache/providers/ICacheProvider.cfc` — declares clearStatistics()
  - low `system/cache/providers/LuceeProvider.cfc` — declares clearStatistics()
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares clearStatistics()
  - … 2 more
- `method 'getCacheStats' has no component return type → system/cache/providers/BoxLangProvider.cfc` — 1 finding(s), 14 candidate(s):
  - low `system/cache/providers/BoxLangProvider.cfc` — declares getSize()
  - low `system/cache/providers/CFProvider.cfc` — declares getSize()
  - low `system/cache/providers/CacheBoxProvider.cfc` — declares getSize()
  - low `system/cache/providers/ICacheProvider.cfc` — declares getSize()
  - low `system/cache/providers/LuceeProvider.cfc` — declares getSize()
  - low `system/cache/providers/MockProvider.cfc` — declares getSize()
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares getSize()
  - low `system/cache/store/BlackHoleStore.cfc` — declares getSize()
  - … 6 more
- `method 'getClassMappingHelper' has no component return type → system/core/util/BoxLangMappingHelper.cfc` — 1 finding(s), 4 candidate(s):
  - low `system/core/util/BoxLangMappingHelper.cfc` — declares addCustomTagPath()
  - low `system/core/util/CFMappingHelper.cfc` — declares addCustomTagPath()
  - low `system/core/util/LuceeMappingHelper.cfc` — declares addCustomTagPath()
  - low `system/core/util/Util.cfc` — declares addCustomTagPath()
- `method 'getColdBoxDSL' has no component return type → system/ioc/dsl/ColdBoxDSL.cfc` — 1 finding(s), 11 candidate(s):
  - medium `system/ioc/dsl/ColdBoxDSL.cfc` — declares process(); named like the receiver 'ColdBoxDSL'
  - low `system/ioc/config/Binder.cfc` — declares process()
  - low `system/ioc/config/Mapping.cfc` — declares process()
  - low `system/ioc/dsl/CacheBoxDSL.cfc` — declares process()
  - low `system/ioc/dsl/IDSLBuilder.cfc` — declares process()
  - low `system/ioc/dsl/LogBoxDSL.cfc` — declares process()
  - low `system/remote/ColdboxProxy.cfc` — declares process()
  - low `system/core/events/EventPool.cfc` — declares process()
  - … 3 more
- `method 'getColdBoxVirtualApp' has no component return type → system/cache/AbstractCacheBoxProvider.cfc` — 1 finding(s), 15 candidate(s):
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares shutdown()
  - low `system/cache/CacheFactory.cfc` — declares shutdown()
  - low `system/ioc/IInjector.cfc` — declares shutdown()
  - low `system/ioc/Injector.cfc` — declares shutdown()
  - low `system/logging/AbstractAppender.cfc` — declares shutdown()
  - low `system/logging/LogBox.cfc` — declares shutdown()
  - low `system/testing/VirtualApp.cfc` — declares shutdown()
  - low `system/async/executors/Executor.cfc` — declares shutdown()
  - … 7 more
- `method 'getConfig' in coldbox.system.logging.LogBox has no component return type → test-harness/models/lib/Config.cfc` — 1 finding(s), 16 candidate(s):
  - medium `test-harness/models/lib/Config.cfc` — declares getMemento(); named like the receiver 'Config'
  - low `system/web/Controller.cfc` — declares getMemento()
  - low `system/web/context/ExceptionBean.cfc` — declares getMemento()
  - low `system/web/context/RequestContext.cfc` — declares getMemento()
  - low `system/web/context/Response.cfc` — declares getMemento()
  - low `system/web/routing/Router.cfc` — declares getMemento()
  - low `system/aop/Matcher.cfc` — declares getMemento()
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares getMemento()
  - … 8 more
- `method 'getExecutor' has no component return type → system/async/executors/Executor.cfc` — 1 finding(s), 8 candidate(s):
  - medium `system/async/executors/Executor.cfc` — declares getStats(); named like the receiver 'Executor'
  - low `system/async/tasks/ScheduledTask.cfc` — declares getStats()
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares getStats()
  - low `system/cache/providers/BoxLangProvider.cfc` — declares getStats()
  - low `system/cache/providers/CFProvider.cfc` — declares getStats()
  - low `system/cache/providers/CacheBoxProvider.cfc` — declares getStats()
  - low `system/cache/providers/ICacheProvider.cfc` — declares getStats()
  - low `system/cache/providers/LuceeProvider.cfc` — declares getStats()
- `method 'getInstance' has no component return type → system/ioc/IProvider.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/ioc/IProvider.cfc` — declares $get()
  - low `system/ioc/Provider.cfc` — declares $get()
- `method 'getInstance' in coldbox.system.ioc.Injector has no component return type → system/cache/AbstractCacheBoxProvider.cfc` — 1 finding(s), 15 candidate(s):
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares setName()
  - low `system/ioc/Injector.cfc` — declares setName()
  - low `system/ioc/Provider.cfc` — declares setName()
  - low `system/logging/AbstractAppender.cfc` — declares setName()
  - low `system/async/executors/Executor.cfc` — declares setName()
  - low `system/async/tasks/ScheduledTask.cfc` — declares setName()
  - low `system/async/tasks/Scheduler.cfc` — declares setName()
  - low `system/cache/providers/ICacheProvider.cfc` — declares setName()
  - … 7 more
- `method 'getJavaCollections' has no component return type → test-harness/models/PhotosService.cfc` — 1 finding(s), 3 candidate(s):
  - low `test-harness/models/PhotosService.cfc` — declares list()
  - low `test-harness/modules_app/resourcesTest/models/PhotosService.cfc` — declares list()
  - low `tests/perf-harness/app/handlers/Api.cfc` — declares list()
- `method 'getJavaNow' in ScheduledTask has no component return type → system/ioc/config/Mapping.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/ioc/config/Mapping.cfc` — declares getValue()
  - low `system/web/context/RequestContext.cfc` — declares getValue()
- `method 'getJavaSystem' has no component return type → system/core/delegates/Env.cfc` — 1 finding(s), 3 candidate(s):
  - low `system/core/delegates/Env.cfc` — declares getEnv()
  - low `system/FrameworkSupertype.cfc` — declares getEnv()
  - low `system/testing/BaseTestCase.cfc` — declares getEnv()
- `method 'getLogBoxDSL' has no component return type → system/ioc/dsl/LogBoxDSL.cfc` — 1 finding(s), 11 candidate(s):
  - medium `system/ioc/dsl/LogBoxDSL.cfc` — declares process(); named like the receiver 'LogBoxDSL'
  - low `system/ioc/config/Binder.cfc` — declares process()
  - low `system/ioc/config/Mapping.cfc` — declares process()
  - low `system/ioc/dsl/CacheBoxDSL.cfc` — declares process()
  - low `system/ioc/dsl/ColdBoxDSL.cfc` — declares process()
  - low `system/ioc/dsl/IDSLBuilder.cfc` — declares process()
  - low `system/remote/ColdboxProxy.cfc` — declares process()
  - low `system/core/events/EventPool.cfc` — declares process()
  - … 3 more
- `method 'getSystemTimezone' has no component return type → tests/tmp/User.cfc` — 1 finding(s), 2 candidate(s):
  - low `tests/tmp/User.cfc` — declares getId()
  - low `test-harness/models/entities/User.cfc` — declares getId()
- `method 'getUtility' has no component return type → system/cache/AbstractCacheBoxProvider.cfc` — 1 finding(s), 2 candidate(s):
  - low `system/cache/AbstractCacheBoxProvider.cfc` — declares inThread()
  - low `system/core/util/Util.cfc` — declares inThread()
- `method 'unless' has no component return type → system/FrameworkSupertype.cfc` — 1 finding(s), 4 candidate(s):
  - low `system/FrameworkSupertype.cfc` — declares when()
  - low `test-harness/models/delegates/FlowHelpers.cfc` — declares when()
  - low `system/async/tasks/ScheduledTask.cfc` — declares when()
  - low `system/core/delegates/Flow.cfc` — declares when()

</details>

## Method definitions — a method not found where it was looked for

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 17 | `cache` | 1 | medium | declares cache | `system/cache/providers/BoxLangProvider.cfc:212` not found in extends chain |
| 5 | `command` | 1 | none |  | `tests/perf-harness/PerformanceSuite.cfc:333` no qualifier, not in file |
| 4 | `getobjectstate` | 1 | medium | declares getObjectState; beside the calling file | `tests/suites/async/performance-parallel-tests.cfm:58` method 'getObjectState' not found in tests.tmp.User |
| 4 | `getrenderedcontent` | 1 | medium | declares getRenderedContent | `tests/specs/integration/EventExecutionsSpec.cfc:70` method 'getRenderedContent' not found in coldbox.system.web.context.RequestContext |
| 4 | `getsetting` | 2 | low | declares getSetting (2 candidates) | `system/modules/HTMLHelper/models/HTMLHelper.cfc:87` no qualifier, not in file |
| 3 | `getnormalizedid` | 1 | medium | declares getNormalizedId | `system/cache/store/indexers/JDBCMetadataIndexer.cfc:98` method 'getNormalizedID' not found in coldbox.system.cache.store.indexers.JDBCMetadataIndexer |
| 3 | `injectstate` | 1 | medium | declares injectState; beside the calling file | `tests/suites/async/performance-parallel-tests.cfm:30` method 'injectState' not found in tests.tmp.User |
| 3 | `isinstancecheck` | 1 | medium | declares isInstanceCheck; beside the calling file | `tests/specs/core/util/UtilTest.cfc:17` method 'isInstanceCheck' not found in coldbox.system.core.util.Util |
| 2 | `getappstarthandlerfired` | 0 | none |  | `tests/specs/web/ControllerTest.cfc:75` method 'getAppStartHandlerFired' not found in coldbox.system.web.Controller |
| 2 | `getcategories` | 1 | medium | declares getCategories | `tests/specs/logging/LogBoxBuildTests.cfc:32` method 'getConfig' not found in LogBox (chain to 'getCategories') |
| 2 | `getclassmetadata` | 0 | none |  | `system/core/dynamic/ObjectPopulator.cfc:583` no qualifier, not in file |
| 2 | `getvalue` | 2 | low | declares getValue (2 candidates) | `system/testing/BaseTestCase.cfc:760` not found in extends chain |
| 2 | `println` | 1 | none |  | `system/async/executors/Executor.cfc:685` no qualifier, not in file |
| 2 | `setbaseurl` | 1 | medium | declares setBaseURL | `tests/suites/eventCachingCollisions/config/routes.cfm:41` no qualifier, not in file |
| 2 | `setisactive` | 2 | low | declares setIsActive (2 candidates) | `tests/specs/orm-enabled/HTMLHelperSpec.cfc:528` method 'setIsActive' not found in User |
| 2 | `sse` | 1 | medium | declares sse; beside the calling file | `system/web/context/SSEStreamer.cfc:36` no qualifier, not in file |
| 2 | `view` | 2 | low | declares view (2 candidates) | `test-harness/external/testLayouts/ext.cfm:45` no qualifier, not in file |
| 1 | `_ucfirst` | 0 | none |  | `system/core/delegates/StringUtil.cfc:127` no qualifier, not in file |
| 1 | `addasset` | 2 | low | declares addAsset (2 candidates) | `test-harness/external/testLayouts/ext.cfm:1` no qualifier, not in file |
| 1 | `addroute` | 1 | medium | declares addRoute | `tests/suites/eventCachingCollisions/config/routes.cfm:111` no qualifier, not in file |
| 1 | `aiagent` | 0 | none |  | `test-harness/config/Router.cfc:20` not found in extends chain |
| 1 | `aigatewayregistry` | 0 | none |  | `system/web/routing/Router.cfc:3064` no qualifier, not in file |
| 1 | `aitool` | 0 | none |  | `test-harness/config/Router.cfc:13` not found in extends chain |
| 1 | `debug` | 5 | low | declares debug (3 candidates) | `system/testing/CustomMatchers.cfc:134` no qualifier, not in file |
| 1 | `expect` | 1 | none |  | `system/testing/CustomMatchers.cfc:89` no qualifier, not in file |
| 1 | `getinstance` | 8 | low | declares getInstance (7 candidates) | `system/modules/HTMLHelper/helpers/Mixins.cfm:9` no qualifier, not in file |
| 1 | `getmodulelist` | 0 | none |  | `system/web/routing/Router.cfc:2451` no qualifier, not in file |
| 1 | `getprivatevalue` | 1 | medium | declares getPrivateValue | `system/testing/BaseTestCase.cfc:778` not found in extends chain |
| 1 | `getrandom` | 1 | medium | declares getRandom | `test-harness/modules_app/resourcesTest/config/Scheduler.cfc:9` method 'getRandom' not found in PhotosService |
| 1 | `getvaluesss` | 0 | none |  | `test-harness/handlers/testerror.cfc:32` method 'getValuesss' not found in coldbox.system.web.context.RequestContext |
| 1 | `includeroutes` | 0 | none |  | `system/web/routing/Router.cfc:358` no qualifier, not in file |
| 1 | `influence` | 0 | none |  | `system/ioc/config/Binder.cfc:521` no qualifier, not in file |
| 1 | `mcpserver` | 0 | none |  | `test-harness/config/Router.cfc:11` not found in extends chain |
| 1 | `objectdeserialize` | 0 | none |  | `system/core/conversion/ObjectMarshaller.cfc:82` no qualifier, not in file |
| 1 | `objectserialize` | 0 | none |  | `system/core/conversion/ObjectMarshaller.cfc:63` no qualifier, not in file |
| 1 | `onshutdown` | 6 | low | declares onShutdown (6 candidates) | `system/logging/LogBox.cfc:211` method 'onShutdown' not found in LogBoxConfig |
| 1 | `ormgethibernateversion` | 0 | none |  | `system/core/util/Util.cfc:503` no qualifier, not in file |
| 1 | `parseandload` | 0 | none |  | `system/web/services/LoaderService.cfc:233` method 'parseAndLoad' not found in coldbox.system.cache.config.CacheBoxConfig |
| 1 | `pathinfoprovider` | 1 | medium | declares pathInfoProvider | `system/web/services/RoutingService.cfc:757` method 'pathInfoProvider' not found in Router |
| 1 | `rendercachecontentreport` | 1 | medium | declares renderCacheContentReport | `system/cache/report/skins/default/CacheReport.cfm:125` no qualifier, not in file |
| 1 | `rendercachereport` | 1 | medium | declares renderCacheReport | `system/cache/report/skins/default/CachePanel.cfm:96` no qualifier, not in file |
| 1 | `run` | 83 | low | declares run (80 candidates) | `tests/index.cfm:29` no qualifier, not in file |
| 1 | `setappstarthandlerfired` | 0 | none |  | `tests/specs/web/ControllerTest.cfc:76` method 'setAppStartHandlerFired' not found in coldbox.system.web.Controller |
| 1 | `setenabled` | 2 | low | declares setEnabled (2 candidates) | `tests/suites/eventCachingCollisions/config/routes.cfm:23` no qualifier, not in file |
| 1 | `setlogbox` | 8 | low | declares setLogBox; beside the calling file (8 candidates) | `system/logging/LogBox.cfc:354` no qualifier, not in file |
| 1 | `showdebugpanel` | 0 | none |  | `tests/suites/eventCachingCollisions/layouts/Layout.None.cfm:2` method 'showdebugpanel' not found in coldbox.system.web.context.RequestContext |
| 1 | `size` | 1 | medium | declares size | `system/cache/store/indexers/JDBCMetadataIndexer.cfc:204` method 'size' not found in coldbox.system.cache.store.indexers.JDBCMetadataIndexer |
| 1 | `this[]` | 0 | none |  | `system/RestHandler.cfc:543` not found in extends chain |

<details><summary>Groups with several candidates</summary>

- `getsetting` — 4 finding(s), 2 candidate(s):
  - low `system/FrameworkSupertype.cfc:317` — declares getSetting
  - low `system/web/Controller.cfc:272` — declares getSetting
- `getvalue` — 2 finding(s), 2 candidate(s):
  - low `system/ioc/config/Mapping.cfc:41` — declares getValue
  - low `system/web/context/RequestContext.cfc:416` — declares getValue
- `setisactive` — 2 finding(s), 2 candidate(s):
  - low `tests/tmp/Person.cfc:11` — declares setIsActive
  - low `test-harness/models/entities/User.cfc:25` — declares setIsActive
- `view` — 2 finding(s), 2 candidate(s):
  - low `system/FrameworkSupertype.cfc:173` — declares view
  - low `system/web/Renderer.cfc:189` — declares view
- `addasset` — 1 finding(s), 2 candidate(s):
  - low `system/modules/HTMLHelper/helpers/Mixins.cfm:8` — declares addAsset
  - low `system/modules/HTMLHelper/models/HTMLHelper.cfc:77` — declares addAsset
- `debug` — 1 finding(s), 3 candidate(s):
  - low `system/logging/Logger.cfc:270` — declares debug
  - low `system/async/tasks/ScheduledTask.cfc:380` — declares debug
  - low `system/logging/config/LogBoxConfig.cfc:398` — declares debug
- `getinstance` — 1 finding(s), 7 candidate(s):
  - low `system/FrameworkSupertype.cfc:88` — declares getInstance
  - low `system/ioc/IInjector.cfc:35` — declares getInstance
  - low `system/ioc/Injector.cfc:487` — declares getInstance
  - low `system/remote/ColdboxProxy.cfc:268` — declares getInstance
  - low `system/testing/BaseTestCase.cfc:860` — declares getInstance
  - low `test-harness/models/formBean.cfc:29` — declares getInstance
  - low `test-harness/models/formImplicitBean.cfc:21` — declares getInstance
- `onshutdown` — 1 finding(s), 6 candidate(s):
  - low `system/async/tasks/Scheduler.cfc:284` — declares onShutdown
  - low `system/web/services/BaseService.cfc:41` — declares onShutdown
  - low `system/web/services/ModuleService.cfc:117` — declares onShutdown
  - low `system/web/services/SchedulerService.cfc:55` — declares onShutdown
  - low `test-harness/config/Scheduler.cfc:71` — declares onShutdown
  - low `tests/suites/loadtests/beload/config/Scheduler.cfc:31` — declares onShutdown
- `run` — 1 finding(s), 80 candidate(s):
  - low `tests/perf-harness/PerformanceSuite.cfc:149` — declares run
  - low `tests/specs/EventHandlerTest.cfc:58` — declares run
  - low `tests/specs/FrameworkSuperTypeTest.cfc:5` — declares run
  - low `tests/specs/RestHandlerTest.cfc:15` — declares run
  - low `build/Build.cfc:131` — declares run
  - low `tests/specs/async/AsyncManagerSpec.cfc:10` — declares run
  - low `tests/specs/async/ExecutorServicesSpec.cfc:14` — declares run
  - low `tests/specs/cache/CacheBoxBuildTests.cfc:12` — declares run
  - … 72 more
- `setenabled` — 1 finding(s), 2 candidate(s):
  - low `system/cache/AbstractCacheBoxProvider.cfc:26` — declares setEnabled
  - low `system/web/routing/Router.cfc:67` — declares setEnabled
- `setlogbox` — 1 finding(s), 8 candidate(s):
  - low `system/logging/AbstractAppender.cfc:55` — declares setLogBox; beside the calling file
  - low `system/FrameworkSupertype.cfc:19` — declares setLogBox
  - low `system/cache/CacheFactory.cfc:41` — declares setLogbox
  - low `system/ioc/Builder.cfc:17` — declares setLogBox
  - low `system/ioc/Injector.cfc:80` — declares setLogBox
  - low `system/web/Controller.cfc:72` — declares setLogbox
  - low `system/ioc/dsl/LogBoxDSL.cfc:17` — declares setLogBox
  - low `system/web/routing/Router.cfc:36` — declares setLogBox

</details>

## Object references — a component path that names no file

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 5 | `chained on 'command', which is not found (calling 'run')` | 83 | none |  | `tests/perf-harness/PerformanceSuite.cfc:333` chained on 'command', which is not found (calling 'run') |
| 5 | `component 'testbox.system.modules.mockdatacfc.models.MockData' does not exist (calling 'mock')` | 1 | none |  | `tests/suites/async/performance-parallel-tests.cfm:16` component 'testbox.system.modules.mockdatacfc.models.MockData' does not exist (calling 'mock') |
| 4 | `component 'docbox.DocBox' does not exist (calling 'generate')` | 0 | none |  | `apidocs/cachebox.cfm:16` component 'docbox.DocBox' does not exist (calling 'generate') |
| 2 | `chained on 'cache', which is not found (calling 'getOrDefault')` | 0 | none |  | `system/cache/providers/BoxLangProvider.cfc:270` chained on 'cache', which is not found (calling 'getOrDefault') |
| 2 | `chained on 'command', which is not found (calling 'flags')` | 1 | none |  | `tests/perf-harness/PerformanceSuite.cfc:348` chained on 'command', which is not found (calling 'flags') |
| 2 | `chained on 'command', which is not found (calling 'params')` | 1 | none |  | `tests/perf-harness/PerformanceSuite.cfc:348` chained on 'command', which is not found (calling 'params') |
| 2 | `component 'bxModules.bxai.models.gateway.http.GatewayRequestProcessor' does not exist (calling 'processInbound')` | 0 | none |  | `system/web/routing/Router.cfc:2973` component 'bxModules.bxai.models.gateway.http.GatewayRequestProcessor' does not exist (calling 'processInbound') |
| 1 | `chained on 'MCPServer', which is not found (calling 'registerTool')` | 0 | none |  | `test-harness/config/Router.cfc:11` chained on 'MCPServer', which is not found (calling 'registerTool') |
| 1 | `chained on 'aiGatewayRegistry', which is not found (calling 'listGateways')` | 0 | none |  | `system/web/routing/Router.cfc:3064` chained on 'aiGatewayRegistry', which is not found (calling 'listGateways') |
| 1 | `chained on 'cache', which is not found (calling 'clearAll')` | 16 | none |  | `system/cache/providers/BoxLangProvider.cfc:427` chained on 'cache', which is not found (calling 'clearAll') |
| 1 | `chained on 'cache', which is not found (calling 'clearQuiet')` | 7 | none |  | `system/cache/providers/BoxLangProvider.cfc:457` chained on 'cache', which is not found (calling 'clearQuiet') |
| 1 | `chained on 'cache', which is not found (calling 'clearStats')` | 0 | none |  | `system/cache/providers/BoxLangProvider.cfc:212` chained on 'cache', which is not found (calling 'clearStats') |
| 1 | `chained on 'cache', which is not found (calling 'getCachedObjectMetadata')` | 13 | none |  | `system/cache/providers/BoxLangProvider.cfc:261` chained on 'cache', which is not found (calling 'getCachedObjectMetadata') |
| 1 | `chained on 'cache', which is not found (calling 'getKeys')` | 13 | none |  | `system/cache/providers/BoxLangProvider.cfc:252` chained on 'cache', which is not found (calling 'getKeys') |
| 1 | `chained on 'cache', which is not found (calling 'getObjectStore')` | 7 | none |  | `system/cache/providers/BoxLangProvider.cfc:221` chained on 'cache', which is not found (calling 'getObjectStore') |
| 1 | `chained on 'cache', which is not found (calling 'getOrSet')` | 4 | none |  | `system/cache/providers/BoxLangProvider.cfc:327` chained on 'cache', which is not found (calling 'getOrSet') |
| 1 | `chained on 'cache', which is not found (calling 'getQuiet')` | 13 | none |  | `system/cache/providers/BoxLangProvider.cfc:279` chained on 'cache', which is not found (calling 'getQuiet') |
| 1 | `chained on 'cache', which is not found (calling 'getSize')` | 14 | none |  | `system/cache/providers/BoxLangProvider.cfc:408` chained on 'cache', which is not found (calling 'getSize') |
| 1 | `chained on 'cache', which is not found (calling 'getStoreMetadataReport')` | 7 | none |  | `system/cache/providers/BoxLangProvider.cfc:228` chained on 'cache', which is not found (calling 'getStoreMetadataReport') |
| 1 | `chained on 'cache', which is not found (calling 'lookup')` | 14 | none |  | `system/cache/providers/BoxLangProvider.cfc:297` chained on 'cache', which is not found (calling 'lookup') |
| 1 | `chained on 'cache', which is not found (calling 'lookupQuiet')` | 7 | none |  | `system/cache/providers/BoxLangProvider.cfc:306` chained on 'cache', which is not found (calling 'lookupQuiet') |
| 1 | `chained on 'cache', which is not found (calling 'reap')` | 13 | none |  | `system/cache/providers/BoxLangProvider.cfc:417` chained on 'cache', which is not found (calling 'reap') |
| 1 | `chained on 'getInstance', which is not found (calling 'addAsset')` | 2 | none |  | `system/modules/HTMLHelper/helpers/Mixins.cfm:9` chained on 'getInstance', which is not found (calling 'addAsset') |
| 1 | `chained on 'setLogBox', which is not found (calling 'onRegistration')` | 6 | none |  | `system/logging/LogBox.cfc:354` chained on 'setLogBox', which is not found (calling 'onRegistration') |
| 1 | `chained on 'setLogBox', which is not found (calling 'setColdBox')` | 14 | none |  | `system/logging/LogBox.cfc:354` chained on 'setLogBox', which is not found (calling 'setColdBox') |
| 1 | `chained on 'setLogBox', which is not found (calling 'setInitialized')` | 1 | none |  | `system/logging/LogBox.cfc:354` chained on 'setLogBox', which is not found (calling 'setInitialized') |
| 1 | `chained on 'setLogBox', which is not found (calling 'setWireBox')` | 13 | none |  | `system/logging/LogBox.cfc:354` chained on 'setLogBox', which is not found (calling 'setWireBox') |
| 1 | `component 'bxModules.bxai.models.gateway.http.GatewayRequestProcessor' does not exist (calling 'processHandshake')` | 0 | none |  | `system/web/routing/Router.cfc:2960` component 'bxModules.bxai.models.gateway.http.GatewayRequestProcessor' does not exist (calling 'processHandshake') |
| 1 | `component 'bxModules.bxai.models.gateway.http.GatewayRequestProcessor' does not exist (calling 'readInteraction')` | 0 | none |  | `system/web/routing/Router.cfc:3010` component 'bxModules.bxai.models.gateway.http.GatewayRequestProcessor' does not exist (calling 'readInteraction') |
| 1 | `component 'bxModules.bxai.models.gateway.http.GatewayRequestProcessor' does not exist (calling 'submitDecision')` | 0 | none |  | `system/web/routing/Router.cfc:3036` component 'bxModules.bxai.models.gateway.http.GatewayRequestProcessor' does not exist (calling 'submitDecision') |
| 1 | `component 'bxModules.bxai.models.mcp.MCPRequestProcessor' does not exist (calling 'processHttp')` | 0 | none |  | `system/web/routing/Router.cfc:2824` component 'bxModules.bxai.models.mcp.MCPRequestProcessor' does not exist (calling 'processHttp') |
| 1 | `component 'cbModuleTest1.models.TestService' does not exist (calling 'sayHello')` | 5 | low | file named TestService.cfc (2 candidates) | `test-harness/modules_app/test-bundle/test1/handlers/test.cfc:23` component 'cbModuleTest1.models.TestService' does not exist (calling 'sayHello') |
| 1 | `component 'coldbox.system.Coldbox' does not exist (calling 'isfwReinit')` | 1 | low | file named Coldbox.cfc (4 candidates) | `tests/suites/eventCachingCollisions/Application.cfc:52` component 'coldbox.system.Coldbox' does not exist (calling 'isfwReinit') |
| 1 | `component 'coldbox.system.Coldbox' does not exist (calling 'loadColdbox')` | 2 | low | file named Coldbox.cfc (4 candidates) | `tests/suites/eventCachingCollisions/Application.cfc:39` component 'coldbox.system.Coldbox' does not exist (calling 'loadColdbox') |
| 1 | `component 'coldbox.system.Coldbox' does not exist (calling 'processColdBoxRequest')` | 1 | low | file named Coldbox.cfc (4 candidates) | `tests/suites/eventCachingCollisions/Application.cfc:63` component 'coldbox.system.Coldbox' does not exist (calling 'processColdBoxRequest') |
| 1 | `component 'coldbox.system.Coldbox' does not exist (calling 'reloadChecks')` | 1 | low | file named Coldbox.cfc (4 candidates) | `tests/suites/eventCachingCollisions/Application.cfc:59` component 'coldbox.system.Coldbox' does not exist (calling 'reloadChecks') |

<details><summary>Groups with several candidates</summary>

- `component 'cbModuleTest1.models.TestService' does not exist (calling 'sayHello')` — 1 finding(s), 2 candidate(s):
  - low `test-harness/modules_app/test-bundle/test1/models/TestService.cfc` — file named TestService.cfc
  - low `test-harness/models/testService.cfc` — file named TestService.cfc
- `component 'coldbox.system.Coldbox' does not exist (calling 'isfwReinit')` — 1 finding(s), 4 candidate(s):
  - low `tests/suites/eventCachingCollisions/config/Coldbox.cfc` — file named Coldbox.cfc
  - low `tests/suites/loadtests/beload/config/Coldbox.cfc` — file named Coldbox.cfc
  - low `test-harness/config/Coldbox.cfc` — file named Coldbox.cfc
  - low `tests/perf-harness/app/config/ColdBox.cfc` — file named Coldbox.cfc
- `component 'coldbox.system.Coldbox' does not exist (calling 'loadColdbox')` — 1 finding(s), 4 candidate(s):
  - low `tests/suites/eventCachingCollisions/config/Coldbox.cfc` — file named Coldbox.cfc
  - low `tests/suites/loadtests/beload/config/Coldbox.cfc` — file named Coldbox.cfc
  - low `test-harness/config/Coldbox.cfc` — file named Coldbox.cfc
  - low `tests/perf-harness/app/config/ColdBox.cfc` — file named Coldbox.cfc
- `component 'coldbox.system.Coldbox' does not exist (calling 'processColdBoxRequest')` — 1 finding(s), 4 candidate(s):
  - low `tests/suites/eventCachingCollisions/config/Coldbox.cfc` — file named Coldbox.cfc
  - low `tests/suites/loadtests/beload/config/Coldbox.cfc` — file named Coldbox.cfc
  - low `test-harness/config/Coldbox.cfc` — file named Coldbox.cfc
  - low `tests/perf-harness/app/config/ColdBox.cfc` — file named Coldbox.cfc
- `component 'coldbox.system.Coldbox' does not exist (calling 'reloadChecks')` — 1 finding(s), 4 candidate(s):
  - low `tests/suites/eventCachingCollisions/config/Coldbox.cfc` — file named Coldbox.cfc
  - low `tests/suites/loadtests/beload/config/Coldbox.cfc` — file named Coldbox.cfc
  - low `test-harness/config/Coldbox.cfc` — file named Coldbox.cfc
  - low `tests/perf-harness/app/config/ColdBox.cfc` — file named Coldbox.cfc

</details>
