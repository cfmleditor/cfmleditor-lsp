# lucee — unresolved findings and candidates

Every finding below is still reported. A candidate is a liberal match offered for a person to weigh, never an answer the resolver took: **high** is the only component declaring every method the function calls on the receiver (or the only one, named like it); **medium** is a sole match on weaker evidence, or the one named like the receiver among several; **low** is one of several. *Defined* is how many indexed files declare a method of that name: 0 means it is missing from the workspace, more means the resolver could not connect the call to it.

| Category | Findings | high | medium | low | none | method defined nowhere |
|---|---:|---:|---:|---:|---:|---:|
| variable | 1061 | 55 | 32 | 111 | 863 | 635 |
| return-type | 17 | 0 | 3 | 2 | 12 | 12 |
| method | 355 | 0 | 106 | 75 | 174 | 167 |
| object | 74 | 0 | 3 | 12 | 59 | 38 |

## Variable definitions — a receiver whose component is unknown

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 141 | `field` | 14 | none |  | `core/src/main/cfml/context/admin/debugging.templates.create.cfm:133` variable 'field' has no component ref |
| 60 | `coll` | 0 | none |  | `test/a_debug_build/_MongoDB.cfc:54` variable 'coll' has no component ref |
| 33 | `driver` | 15 | none |  | `core/src/main/cfml/context/admin/debugging.templates.create.cfm:47` variable 'driver' has no component ref |
| 33 | `res` | 0 | none |  | `test/functions/StructToSorted.cfc:21` variable 'res' has no component ref |
| 30 | `msg` | 1 | none |  | `test/tags/Mail2.cfc:52` variable 'msg' has no component ref |
| 27 | `result` | 0 | none |  | `test/components/Administrator.cfc:1687` variable 'result' has no component ref |
| 20 | `t` | 0 | none |  | `test/functions/SetDay.cfc:22` variable 't' has no component ref |
| 16 | `iter` | 0 | none |  | `test/tickets/LDEV1109.cfc:45` variable 'iter' has no component ref |
| 15 | `field → core/src/main/java/resource/context/admin/dbdriver/types/Field.cfc` | 11 | high | declares getType(), getName(), getDefaultValue(), getDisplayName(), getRequired(), getDefaultValueIndex(), getDescription(); named like the receiver 'field' | `core/src/main/cfml/context/admin/services.datasource.create.cfm:532` variable 'field' has no component ref |
| 15 | `local.date2` | 0 | none |  | `test/functions/DateDiff.cfc:444` variable 'local.date2' has no component ref |
| 15 | `p` | 0 | none |  | `test/tickets/LDEV1109.cfc:77` variable 'p' has no component ref |
| 14 | `arr` | 0 | none |  | `test/jira/Jira2970.cfc:211` variable 'arr' has no component ref |
| 14 | `driver → core/src/main/java/resource/context/admin/mailservers/GMX.cfc` | 8 | low | declares getShortName(), getLabel(), getHost(), getPort(), useSSL(), useTLS() (8 candidates) | `core/src/main/cfml/context/admin/services.mail.form.cfm:67` variable 'driver' has no component ref |
| 13 | `results` | 0 | none |  | `core/src/main/java/resource/component/org/lucee/cfml/test/LuceeTestSuiteRunner.cfc:43` variable 'results' has no component ref |
| 13 | `tz` | 0 | none |  | `test/datasource/Oracle.cfc:44` variable 'tz' has no component ref |
| 12 | `docsfound` | 0 | none |  | `test/a_debug_build/_MongoDB.cfc:166` variable 'docsFound' has no component ref |
| 12 | `sss` | 0 | none |  | `test/general/Resources.cfc:258` variable 'sss' has no component ref |
| 11 | `_cookies` | 0 | none |  | `test/tickets/LDEV3478.cfc:26` variable '_cookies' has no component ref |
| 11 | `config` | 0 | none |  | `core/src/main/cfml/context/admin/messaging.cfm:105` variable 'config' has no component ref |
| 11 | `engine` | 0 | none |  | `test/general/JSR223.cfc:60` variable 'engine' has no component ref |
| 11 | `mapping` | 0 | none |  | `test/general/ZipResource.cfc:27` variable 'mapping' has no component ref |
| 9 | `application.objects.utils → core/src/main/cfml/context/doc/DocUtils.cfc` | 3 | low | declares getAllFunctions() (3 candidates) | `core/src/main/cfml/context/doc/categories.cfm:4` variable 'Application.objects.utils' has no component ref |
| 9 | `local.date3` | 0 | none |  | `test/functions/DateDiff.cfc:464` variable 'local.date3' has no component ref |
| 9 | `mail` | 0 | none |  | `test/tags/Mail2.cfc:47` variable 'mail' has no component ref |
| 9 | `parts` | 2 | none |  | `test/tags/Mail2.cfc:206` variable 'parts' has no component ref |
| 9 | `pc → test/tickets/LDEV4485.cfc` | 1 | medium | declares getConfig() | `core/src/main/cfml/context/doc/Application.cfc:102` variable 'pc' has no component ref |
| 8 | `admin` | 0 | none |  | `core/src/main/cfml/context/admin/messaging.cfm:18` variable 'admin' has no component ref |
| 8 | `db` | 0 | none |  | `test/a_debug_build/_MongoDB.cfc:45` variable 'db' has no component ref |
| 8 | `it` | 0 | none |  | `test/tags/_Mail.cfc:57` variable 'it' has no component ref |
| 8 | `out` | 0 | none |  | `test/functions/configTranslate.cfc:19` variable 'out' has no component ref |
| 8 | `s` | 0 | none |  | `test/general/Resources.cfc:278` variable 's' has no component ref |
| 7 | `db[]` | 0 | none |  | `test/a_debug_build/_MongoDB.cfc:115` variable 'db[]' has no component ref |
| 7 | `m` | 0 | none |  | `core/src/main/cfml/context/doc/Application.cfc:107` variable 'm' has no component ref |
| 7 | `o` | 1 | none |  | `test/general/Resources.cfc:379` variable 'o' has no component ref |
| 7 | `p → test/tickets/LDEV3102.cfc` | 10 | low | declares getDatasource() (10 candidates) | `test/tickets/LDEV5963.cfc:68` variable 'p' has no component ref |
| 7 | `testcase` | 1 | none |  | `core/src/main/java/resource/component/org/lucee/cfml/test/LuceeTestSuiteRunner.cfc:21` variable 'testCase' has no component ref |
| 6 | `c → core/src/main/java/resource/component/org/lucee/cfml/Administrator.cfc` | 3 | high | declares getMappings(), getComponentMappings(), getCustomTagMappings() | `test/general/Mappings.cfc:53` variable 'c' has no component ref |
| 6 | `froms[]` | 0 | none |  | `test/tags/Mail2.cfc:54` variable 'froms[]' has no component ref |
| 6 | `map` | 0 | none |  | `test/tickets/LDEV0866.cfc:34` variable 'map' has no component ref |
| 6 | `myarray` | 0 | none |  | `test/tickets/LDEV3026.cfc:7` variable 'myArray' has no component ref |
| 6 | `rtn → test/jira/Jira1460.cfc` | 5 | low | declares a(), b() (5 candidates) | `test/jira/Jira2633/Test.cfc:40` variable 'rtn' has no component ref |
| 6 | `tos[]` | 0 | none |  | `test/tags/Mail2.cfc:59` variable 'tos[]' has no component ref |
| 5 | `application.adminfunctions → core/src/main/cfml/context/admin/adminfunctions.cfc` | 1 | high | declares canAccessContext(), isFavorite(); named like the receiver 'adminfunctions' | `core/src/main/cfml/context/admin/admin_layout.cfm:71` variable 'application.adminfunctions' has no component ref |
| 5 | `c` | 0 | none |  | `test/general/Resources.cfc:384` variable 'c' has no component ref |
| 5 | `cd` | 3 | none |  | `test/_setupTestServices.cfc:847` variable 'cd' has no component ref |
| 5 | `class2` | 0 | none |  | `test/tickets/LDEV6226.cfc:80` variable 'class2' has no component ref |
| 5 | `drivers[] → core/src/main/java/resource/context/admin/mailservers/GMX.cfc` | 8 | low | declares getHost(), getPort(), useSSL(), useTLS(), getLabel() (8 candidates) | `core/src/main/cfml/context/admin/services.mail.serverlist.cfm:12` variable 'drivers[]' has no component ref |
| 5 | `e` | 0 | none |  | `test/tickets/LDEV0866.cfc:48` variable 'e' has no component ref |
| 5 | `file` | 0 | none |  | `core/src/main/cfml/context/gateway/WatchService.cfc:29` variable 'file' has no component ref |
| 5 | `qry` | 0 | none |  | `test/tickets/LDEV1280.cfc:25` variable 'qry' has no component ref |
| 5 | `src.debugtemplates` | 0 | none |  | `test/tickets/LDEV5758.cfc:579` variable 'src.debugTemplates' has no component ref |
| 4 | `arguments.spec` | 0 | none |  | `test/tickets/LDEV0359.cfc:37` variable 'arguments.spec' has no component ref |
| 4 | `body` | 0 | none |  | `ant/upload_to_s3.cfm:170` variable 'body' has no component ref |
| 4 | `callercache` | 0 | none |  | `test/tickets/LDEV6240.cfc:170` variable 'callerCache' has no component ref |
| 4 | `d` | 0 | none |  | `test/functions/DateTimeFormat.cfc:31` variable 'd' has no component ref |
| 4 | `d2` | 0 | none |  | `test/functions/DateDiff.cfc:324` variable 'd2' has no component ref |
| 4 | `driver → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` | 40 | low | declares getLabel(), getClass() (21 candidates) | `core/src/main/cfml/context/admin/services.cache.list.cfm:135` variable 'driver' has no component ref |
| 4 | `driver → core/src/main/java/resource/context/admin/aidriver/OpenAI.cfc` | 40 | high | declares getLabel(), getClass(), getLabelLong(), getDescription() | `core/src/main/cfml/context/admin/services.ai.list.cfm:94` variable 'driver' has no component ref |
| 4 | `drivers[] → core/src/main/java/resource/context/admin/debug/Classic.cfc` | 48 | low | declares getDescription(), getId(), getLabel() (4 candidates) | `core/src/main/cfml/context/admin/debugging.logs.detail.cfm:72` variable 'drivers[]' has no component ref |
| 4 | `field → core/src/main/java/resource/context/admin/logging/appender/Field.cfc` | 7 | high | declares getName(), getDefaultValue(), setDefaultValue(); named like the receiver 'field' | `core/src/main/java/resource/context/admin/logging/appender/DatasourceAppender.cfc:25` variable 'field' has no component ref |
| 4 | `future` | 0 | none |  | `test/tickets/LDEV1841.cfc:44` variable 'future' has no component ref |
| 4 | `httpsession → test/tickets/LDEV0078/Test.cfc` | 96 | low | declares getId() (96 candidates) | `test/tickets/LDEV5942/jee-session/rotate-with-data.cfm:14` variable 'httpSession' has no component ref |
| 4 | `notexa.b.c → test/general/InlineComponent.cfc` | 2 | low | declares d() (2 candidates) | `test/general/Elvis.cfc:65` variable 'notexa.b.c' has no component ref |
| 4 | `st` | 0 | none |  | `test/tags/processingDirective/cfprocessingdirective_preservecase.cfm:8` variable 'st' has no component ref |
| 4 | `tz → test/jira/Jira2275/orm/FieldLink.cfc` | 14 | low | declares getDisplayName() (14 candidates) | `test/tickets/LDEV1761.cfc:38` variable 'tz' has no component ref |
| 3 | `a → test/general/InlineComponent.cfc` | 6 | low | declares b() (5 candidates) | `test/general/SafeNavigator.cfc:7` variable 'a' has no component ref |
| 3 | `a.b.c → test/general/InlineComponent.cfc` | 2 | low | declares d() (2 candidates) | `test/general/Elvis.cfc:80` variable 'a.b.c' has no component ref |
| 3 | `afterstored` | 2 | none |  | `test/tickets/LDEV5941/testCommitNoMetadataBump.cfm:19` variable 'afterStored' has no component ref |
| 3 | `appender → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` | 40 | low | declares getLabel(), getClass() (21 candidates) | `core/src/main/cfml/context/admin/server.logging.list.cfm:97` variable 'appender' has no component ref |
| 3 | `arguments.path` | 1 | none |  | `core/src/main/cfml/context/gateway/WatchService.cfc:20` variable 'arguments.path' has no component ref |
| 3 | `attachment` | 0 | none |  | `test/tags/Mail2.cfc:264` variable 'attachment' has no component ref |
| 3 | `before` | 2 | none |  | `test/tickets/LDEV5941/testCommitNoMetadataBump.cfm:12` variable 'before' has no component ref |
| 3 | `bundle` | 0 | none |  | `test/tickets/LDEV6088.cfc:26` variable 'bundle' has no component ref |
| 3 | `cfc → core/src/main/cfml/context/admin/ExtensionProviderProxy.cfc` | 1 | high | declares listApplications(), getInfo() | `core/src/main/cfml/context/admin/ExtensionProviderProxy.cfc:60` variable 'cfc' has no component ref |
| 3 | `content` | 2 | none |  | `test/tags/Mail2.cfc:255` variable 'content' has no component ref |
| 3 | `date` | 0 | none |  | `test/tickets/LDEV1041.cfc:23` variable 'date' has no component ref |
| 3 | `detail.provider → core/src/main/cfml/context/admin/ExtensionProviderProxy.cfc` | 1 | high | declares listApplications(), getInfo() | `core/src/main/cfml/context/admin/ext.functions.cfm:100` variable 'detail.provider' has no component ref |
| 3 | `driver → core/src/main/java/resource/context/admin/debug/Classic.cfc` | 5 | low | declares output() (5 candidates) | `core/src/main/cfml/context/admin/debugging.logs.detail.cfm:104` variable 'driver' has no component ref |
| 3 | `email` | 0 | none |  | `test/tags/_Mail.cfc:59` variable 'email' has no component ref |
| 3 | `empdetails` | 0 | none |  | `test/tickets/LDEV1160.cfc:10` variable 'empDetails' has no component ref |
| 3 | `extensions` | 0 | none |  | `test/_setupTestServices.cfc:866` variable 'extensions' has no component ref |
| 3 | `extmeta` | 0 | none |  | `test/tickets/LDEV6088.cfc:19` variable 'extMeta' has no component ref |
| 3 | `factory` | 0 | none |  | `core/src/main/cfml/context/admin/overview.cfm:56` variable 'factory' has no component ref |
| 3 | `form` | 0 | none |  | `test/tags/cache/cache_useQueryString.cfm:6` variable 'form' has no component ref |
| 3 | `hibernatesessionstats` | 0 | none |  | `test/tickets/LDEV4339/ormSessionCheck.cfm:3` variable 'HibernateSessionStats' has no component ref |
| 3 | `jsonobj` | 0 | none |  | `test/tickets/LDEV5530.cfc:35` variable 'jsonObj' has no component ref |
| 3 | `layout → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` | 40 | low | declares getLabel(), getClass() (21 candidates) | `core/src/main/cfml/context/admin/server.logging.list.cfm:98` variable 'layout' has no component ref |
| 3 | `plugin.component → core/src/main/cfml/context/admin/plugin/Plugin.cfc` | 1 | high | declares _action(), action(), _display() | `core/src/main/cfml/context/admin/plugin.cfm:65` variable 'plugin.component' has no component ref |
| 3 | `src.rest` | 0 | none |  | `test/tickets/LDEV5758.cfc:353` variable 'src.rest' has no component ref |
| 2 | `_defaultdriver → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` | 40 | low | declares getLabel() (40 candidates) | `core/src/main/cfml/context/admin/services.mail.form.cfm:76` variable '_DefaultDriver' has no component ref |
| 2 | `antisamyresult` | 0 | none |  | `test/tickets/LDEV5085/AntiSamyService.cfc:20` variable 'antiSamyResult' has no component ref |
| 2 | `arg` | 0 | none |  | `core/src/main/java/resource/component/org/lucee/cfml/Administrator.cfc:2974` variable 'arg' has no component ref |
| 2 | `args` | 0 | none |  | `test/studio/index.cfm:21` variable 'args' has no component ref |
| 2 | `arguments.debugtemplate → core/src/main/java/resource/context/admin/debug/Classic.cfc` | 5 | low | declares output() (5 candidates) | `core/src/main/java/resource/context/admin/info/Info.cfc:31` variable 'arguments.debugTemplate' has no component ref |
| 2 | `arguments.results` | 0 | none |  | `test/tickets/LDEV4892.cfc:34` variable 'arguments.results' has no component ref |
| 2 | `cgi` | 0 | none |  | `test/tickets/LDEV3841/cfc/index.cfm:3` variable 'cgi' has no component ref |
| 2 | `code → test/orm/clearSession/Code.cfc` | 96 | low | declares getId(), getCode(); named like the receiver 'code' (7 candidates) | `test/orm/Hibernate.cfc:52` variable 'code' has no component ref |
| 2 | `code → test/tickets/LDEV0881/Code.cfc` | 96 | low | declares getId(), getCode(); named like the receiver 'code' (7 candidates) | `test/tickets/LDEV0881.cfc:38` variable 'code' has no component ref |
| 2 | `coll → core/src/main/java/resource/component/org/lucee/cfml/Ftp.cfc` | 1 | medium | declares rename() | `test/a_debug_build/_MongoDB.cfc:330` variable 'coll' has no component ref |
| 2 | `componentmappings[]` | 0 | none |  | `test/general/Mappings.cfc:70` variable 'componentMappings[]' has no component ref |
| 2 | `customtagmappings[]` | 0 | none |  | `test/general/Mappings.cfc:81` variable 'customTagMappings[]' has no component ref |
| 2 | `datevalue` | 0 | none |  | `test/tickets/LDEV3789.cfc:16` variable 'dateValue' has no component ref |
| 2 | `entries` | 0 | none |  | `test/tickets/LDEV1109.cfc:44` variable 'entries' has no component ref |
| 2 | `event` | 0 | none |  | `core/src/main/cfml/context/gateway/WatchService.cfc:62` variable 'event' has no component ref |
| 2 | `headers` | 0 | none |  | `test/tags/Mail.cfc:140` variable 'headers' has no component ref |
| 2 | `javaproxies` | 0 | none |  | `test/tickets/LDEV5459.cfc:72` variable 'javaProxies' has no component ref |
| 2 | `loadobj → test/tickets/LDEV0506.cfc` | 2 | high | declares cachedFunction(), nonCachedFunction() | `test/tickets/LDEV0506.cfc:17` variable 'loadObj' has no component ref |
| 2 | `message` | 0 | none |  | `test/tags/Mail.cfc:138` variable 'message' has no component ref |
| 2 | `messages[] → test/tickets/LDEV0752/SupportTicket.cfc` | 1 | medium | declares getSubject() | `test/tags/Mail2.cfc:301` variable 'messages[]' has no component ref |
| 2 | `mongo` | 0 | none |  | `test/a_debug_build/_MongoDB.cfc:106` variable 'mongo' has no component ref |
| 2 | `myvar` | 0 | none |  | `test/general/SafeNavOp.cfc:20` variable 'myvar' has no component ref |
| 2 | `myvar.firstlevel` | 0 | none |  | `test/general/SafeNavOp.cfc:21` variable 'myvar.firstlevel' has no component ref |
| 2 | `myvar.firstlevel.nextlevel` | 0 | none |  | `test/general/SafeNavOp.cfc:22` variable 'myvar.firstlevel.nextlevel' has no component ref |
| 2 | `names` | 0 | none |  | `test/tags/_Mail.cfc:61` variable 'names' has no component ref |
| 2 | `newperson.testclosurethis.mappers` | 0 | none |  | `test/tickets/LDEV4067/LDEV4067.cfm:6` variable 'newPerson.testClosureThis.MAPPERS' has no component ref |
| 2 | `newperson.testclosurevar.mappers` | 0 | none |  | `test/tickets/LDEV4067/LDEV4067.cfm:11` variable 'newPerson.testClosureVar.MAPPERS' has no component ref |
| 2 | `newperson.testlambdathis.mappers` | 0 | none |  | `test/tickets/LDEV4067/LDEV4067.cfm:16` variable 'newPerson.testLambdaThis.MAPPERS' has no component ref |
| 2 | `newperson.testlambdavar.mappers` | 0 | none |  | `test/tickets/LDEV4067/LDEV4067.cfm:21` variable 'newPerson.testLambdaVar.MAPPERS' has no component ref |
| 2 | `odnsdomain → test/tickets/LDEV0078/Test.cfc` | 96 | low | declares getId() (96 candidates) | `test/tickets/LDEV3060/invalidcomponent.cfc:4` variable 'oDnsDomain' has no component ref |
| 2 | `pc` | 0 | none |  | `core/src/main/cfml/context/admin/overview.cfm:55` variable 'pc' has no component ref |
| 2 | `pools` | 0 | none |  | `test/tickets/LDEV6051.cfc:91` variable 'pools' has no component ref |
| 2 | `presideobjectservice` | 1 | none |  | `test/tickets/LDEV4901.cfc:4942` variable 'presideObjectService' has no component ref |
| 2 | `provider` | 0 | none |  | `test/general/Resources.cfc:696` variable 'provider' has no component ref |
| 2 | `qry.driver → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` | 40 | low | declares getLabel() (40 candidates) | `core/src/main/cfml/context/admin/debugging.templates.list.cfm:137` variable 'qry.driver' has no component ref |
| 2 | `ref → test/orm/many2one/Ref.cfc` | 96 | low | declares getId(), getCode(); named like the receiver 'ref' (7 candidates) | `test/orm/Hibernate.cfc:48` variable 'ref' has no component ref |
| 2 | `ref → test/tickets/LDEV0881/Ref.cfc` | 96 | low | declares getId(), getCode(); named like the receiver 'ref' (7 candidates) | `test/tickets/LDEV0881.cfc:34` variable 'ref' has no component ref |
| 2 | `res → test/jira/Jira2644/State.cfc` | 1 | high | declares getStateCode(), getCountryCode() | `test/jira/Jira2644.cfc:34` variable 'res' has no component ref |
| 2 | `res.client[]` | 0 | none |  | `test/tickets/LDEV0215_mysql.cfc:24` variable 'res.client[]' has no component ref |
| 2 | `res.session[]` | 0 | none |  | `test/tickets/LDEV0215_mysql.cfc:20` variable 'res.session[]' has no component ref |
| 2 | `session.ldev3125` | 0 | none |  | `test/tickets/LDEV2135/cfml-session/secondRequest.cfm:7` variable 'session.ldev3125' has no component ref |
| 2 | `sreturn` | 0 | none |  | `test/tickets/LDEV5092.cfc:54` variable 'sReturn' has no component ref |
| 2 | `storedvalue` | 0 | none |  | `test/tickets/LDEV5941/testCommitOnly.cfm:16` variable 'storedValue' has no component ref |
| 2 | `t3[] → test/tickets/LDEV0374/users.cfc` | 1 | medium | declares getDateJoined() | `test/tickets/LDEV0374/test.cfm:24` variable 't3[]' has no component ref |
| 2 | `testqry` | 0 | none |  | `test/tickets/LDEV1289/test.cfm:4` variable 'testQry' has no component ref |
| 2 | `url` | 0 | none |  | `test/tickets/LDEV2374/LDEV2374.cfm:2` variable 'url' has no component ref |
| 2 | `val → test/tickets/LDEV0966/orm/tblitem.cfc` | 11 | low | declares getType() (11 candidates) | `test/tickets/LDEV1109.cfc:52` variable 'val' has no component ref |
| 2 | `variables` | 2 | none |  | `core/src/main/cfml/context/admin/chartProcess.cfm:34` variable 'variables' has no component ref |
| 2 | `wc` | 0 | none |  | `test/a_debug_build/_MongoDB.cfc:256` variable 'wc' has no component ref |
| 1 | `a.b → test/general/InlineComponent.cfc` | 5 | low | declares c() (3 candidates) | `test/general/SafeNavigator.cfc:8` variable 'a.b' has no component ref |
| 1 | `adm → test/tickets/LDEV6066.cfc` | 2 | low | declares removeDatasource() (2 candidates) | `test/tickets/LDEV4827.cfc:52` variable 'adm' has no component ref |
| 1 | `adminurls` | 0 | none |  | `core/src/main/cfml/context/admin/web.cfm:493` variable 'adminUrls' has no component ref |
| 1 | `arguments.a.b.c → test/general/InlineComponent.cfc` | 2 | low | declares d() (2 candidates) | `test/general/FunctionListener.cfc:99` variable 'arguments.a.b.c' has no component ref |
| 1 | `arguments.envcl` | 0 | none |  | `test/tickets/LDEV6240.cfc:73` variable 'arguments.envCL' has no component ref |
| 1 | `arguments.key` | 0 | none |  | `core/src/main/cfml/context/gateway/WatchService.cfc:61` variable 'arguments.key' has no component ref |
| 1 | `arguments.l → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` | 40 | low | declares getLabel() (40 candidates) | `core/src/main/cfml/context/admin/server.logging.cfm:17` variable 'arguments.l' has no component ref |
| 1 | `arguments.name` | 0 | none |  | `test/tickets/LDEV5473.cfc:8` variable 'arguments.name' has no component ref |
| 1 | `arguments.r → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` | 40 | low | declares getLabel() (40 candidates) | `core/src/main/cfml/context/admin/server.logging.cfm:17` variable 'arguments.r' has no component ref |
| 1 | `attributes.b` | 0 | none |  | `test/tickets/LDEV5967.cfc:64` variable 'attributes.b' has no component ref |
| 1 | `bookmark → test/orm/many2many/model/Bookmark.cfc` | 1 | high | declares getTags(); named like the receiver 'bookmark' | `test/orm/many2many/index.cfm:69` variable 'bookmark' has no component ref |
| 1 | `cfcc → test/jira/Jira0713/Test.cfc` | 56 | low | declares getName(), setName() (56 candidates) | `test/jira/Jira0713.cfc:30` variable 'cfcc' has no component ref |
| 1 | `cfg` | 0 | none |  | `test/tickets/LDEV6088.cfc:8` variable 'cfg' has no component ref |
| 1 | `cfthread.ldev5320_getsession` | 0 | none |  | `test/tickets/LDEV5320/cfml-session/threadSession.cfm:21` variable 'cfthread.ldev5320_getSession' has no component ref |
| 1 | `cl` | 0 | none |  | `test/run-tests.cfm:20` variable 'cl' has no component ref |
| 1 | `class1` | 0 | none |  | `test/tickets/LDEV6226.cfc:79` variable 'class1' has no component ref |
| 1 | `clazz` | 0 | none |  | `test/run-tests.cfm:7` variable 'clazz' has no component ref |
| 1 | `config → core/src/main/java/resource/component/org/lucee/cfml/Administrator.cfc` | 1 | medium | declares getComponentMappings() | `core/src/main/cfml/context/doc/Application.cfc:103` variable 'config' has no component ref |
| 1 | `config → test/tickets/LDEV5567/LDEV5660.cfc` | 3 | low | declares getMappings() (2 candidates) | `test/general/ZipResource.cfc:23` variable 'config' has no component ref |
| 1 | `configutil → core/src/main/java/resource/component/org/lucee/cfml/Ftp.cfc` | 1 | medium | declares getFile() | `test/tickets/LDEV-4796.cfc:22` variable 'ConfigUtil' has no component ref |
| 1 | `conn` | 0 | none |  | `test/_setupTestServices.cfc:338` variable 'conn' has no component ref |
| 1 | `cookie` | 0 | none |  | `test/tickets/LDEV5288/ldev5288-createJsession.cfm:6` variable 'cookie' has no component ref |
| 1 | `currdate` | 0 | none |  | `test/tickets/LDEV2044.cfc:7` variable 'currDate' has no component ref |
| 1 | `d → core/src/main/java/resource/context/admin/gdriver/ActiveMQ.cfc` | 6 | low | declares getCFCPath(), getClass() (6 candidates) | `core/src/main/cfml/context/admin/services.gateway.cfm:45` variable 'd' has no component ref |
| 1 | `d1` | 0 | none |  | `test/functions/DatePart.cfc:13` variable 'd1' has no component ref |
| 1 | `data` | 0 | none |  | `core/src/main/cfml/context/gateway/MailWatcherListener.cfc:21` variable 'data' has no component ref |
| 1 | `date2` | 0 | none |  | `test/tickets/LDEV0374/test.cfm:29` variable 'date2' has no component ref |
| 1 | `dd` | 0 | none |  | `test/general/Resources.cfc:417` variable 'dd' has no component ref |
| 1 | `deleted` | 0 | none |  | `test/tickets/LDEV4670/ldev4670.cfm:19` variable 'deleted' has no component ref |
| 1 | `drivers[] → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` | 48 | low | declares getDescription() (48 candidates) | `core/src/main/cfml/context/admin/debugging.templates.list.cfm:82` variable 'drivers[]' has no component ref |
| 1 | `ds` | 0 | none |  | `test/tags/Query.cfc:214` variable 'ds' has no component ref |
| 1 | `entriesfield` | 0 | none |  | `test/tickets/LDEV1109.cfc:37` variable 'entriesField' has no component ref |
| 1 | `foo` | 1 | none |  | `test/tickets/LDEV5353.cfc:17` variable 'foo' has no component ref |
| 1 | `header` | 2 | none |  | `test/tags/Mail.cfc:142` variable 'header' has no component ref |
| 1 | `httpresult.cookies` | 0 | none |  | `test/tickets/LDEV4756.cfc:202` variable 'httpResult.cookies' has no component ref |
| 1 | `httpsendresult → core/src/main/java/resource/component/org/lucee/cfml/Result.cfc` | 1 | medium | declares getPrefix() | `test/tickets/LDEV1020.cfc:30` variable 'httpSendResult' has no component ref |
| 1 | `httpsession → test/tickets/LDEV0233/model/ContentType.cfc` | 96 | high | declares getId(), getAttribute() | `test/tickets/LDEV5942/jee-session/invalidate.cfm:10` variable 'httpSession' has no component ref |
| 1 | `instr` | 0 | none |  | `test/run-tests.cfm:6` variable 'instr' has no component ref |
| 1 | `interfaces[]` | 0 | none |  | `test/tickets/LDEV1121/test.cfm:4` variable 'interfaces[]' has no component ref |
| 1 | `item → test/tickets/LDEV2056/Base.cfc` | 1 | medium | declares getMemento() | `test/tickets/LDEV2056.cfc:19` variable 'item' has no component ref |
| 1 | `java.lang` | 0 | none |  | `test/tickets/LDEV5807.cfc:66` variable 'java.lang' has no component ref |
| 1 | `jcompletor → test/functions/CreateDynamicProxy/osgi/Completor.cfc` | 1 | medium | declares complete() | `test/functions/CreateDynamicProxy/osgi/index.cfm:16` variable 'jCompletor' has no component ref |
| 1 | `jdbcs` | 0 | none |  | `test/_setupTestServices.cfc:873` variable 'jdbcs' has no component ref |
| 1 | `jsess` | 0 | none |  | `core/src/main/cfml/context/admin/Jira.cfc:800` variable 'jsess' has no component ref |
| 1 | `kind → test/tickets/LDEV1818/Sample.cfc` | 4 | low | declares name() (4 candidates) | `core/src/main/cfml/context/gateway/WatchService.cfc:84` variable 'kind' has no component ref |
| 1 | `list` | 0 | none |  | `test/general/StaticMembersInvoke.cfc:40` variable 'list' has no component ref |
| 1 | `ljkl.jljl` | 0 | none |  | `test/general/Elvis.cfc:59` variable 'ljkl.jljl' has no component ref |
| 1 | `local.a.b.c → test/general/InlineComponent.cfc` | 2 | low | declares d() (2 candidates) | `test/general/FunctionListener.cfc:79` variable 'local.a.b.c' has no component ref |
| 1 | `mappings[]` | 0 | none |  | `test/general/Mappings.cfc:59` variable 'mappings[]' has no component ref |
| 1 | `md` | 0 | none |  | `test/_setupTestServices.cfc:868` variable 'md' has no component ref |
| 1 | `ml → test/orm/many2many/model/Bookmark.cfc` | 96 | low | declares getId() (96 candidates) | `test/orm/many2many/index.cfm:40` variable 'ml' has no component ref |
| 1 | `newclass → core/src/main/java/resource/context/admin/dbdriver/Driver.cfc` | 2 | low | declares init(), getValue() (2 candidates) | `test/tickets/LDEV4001/LDEV4001.cfm:18` variable 'newClass' has no component ref |
| 1 | `obj[] → test/tickets/LDEV1741/App1/model/Foo.cfc` | 40 | low | declares getLabel() (40 candidates) | `test/tickets/LDEV1741/App1/index.cfm:14` variable 'obj[]' has no component ref |
| 1 | `obj[] → test/tickets/LDEV1741/App2/model/Foo.cfc` | 40 | low | declares getLabel() (40 candidates) | `test/tickets/LDEV1741/App2/index.cfm:14` variable 'obj[]' has no component ref |
| 1 | `obj[] → test/tickets/LDEV1984/App1/model/Foo.cfc` | 40 | low | declares getLabel() (40 candidates) | `test/tickets/LDEV1984/App1/index.cfm:14` variable 'obj[]' has no component ref |
| 1 | `originalstring` | 0 | none |  | `test/general/String.cfc:12` variable 'originalString' has no component ref |
| 1 | `os` | 0 | none |  | `test/tickets/LDEV5510.cfc:29` variable 'os' has no component ref |
| 1 | `process` | 0 | none |  | `test/tickets/LDEV5510.cfc:26` variable 'process' has no component ref |
| 1 | `prop` | 0 | none |  | `test/tickets/LDEV6004.cfc:12` variable 'prop' has no component ref |
| 1 | `proxiedobject → test/tickets/LDEV-4676.cfc` | 1477 | low | declares run() (1476 candidates) | `test/tickets/LDEV1778.cfc:11` variable 'proxiedObject' has no component ref |
| 1 | `proxy → test/functions/CreateDynamicProxy/javaSetting/Test.cfc` | 2 | low | declares hello() (2 candidates) | `test/functions/CreateDynamicProxy/javaSetting/index.cfm:27` variable 'proxy' has no component ref |
| 1 | `proxy → test/jira/Jira3096/Test.cfc` | 2 | low | declares hello() (2 candidates) | `test/jira/Jira3096/index.cfm:27` variable 'proxy' has no component ref |
| 1 | `ps` | 0 | none |  | `test/general/ZipResource.cfc:28` variable 'ps' has no component ref |
| 1 | `q_sessions` | 0 | none |  | `test/tickets/LDEV4670/ldev4670.cfm:25` variable 'q_sessions' has no component ref |
| 1 | `q_ssorted` | 0 | none |  | `test/tickets/LDEV1294.cfc:29` variable 'q_ssorted' has no component ref |
| 1 | `qoq` | 0 | none |  | `test/tickets/LDEV1525/ldev1525.cfm:26` variable 'qoq' has no component ref |
| 1 | `r → test/tickets/LDEV6138/TestEntity.cfc` | 56 | low | declares getName(), setName() (56 candidates) | `test/tickets/LDEV6138/LDEV6138.cfm:34` variable 'r' has no component ref |
| 1 | `ramcache` | 0 | none |  | `test/tickets/LDEV1109.cfc:36` variable 'ramCache' has no component ref |
| 1 | `rawcause` | 0 | none |  | `test/tickets/LDEV6347.cfc:56` variable 'rawCause' has no component ref |
| 1 | `rce → core/src/main/java/resource/context/admin/dbdriver/Driver.cfc` | 2 | low | declares getValue() (2 candidates) | `test/tickets/LDEV1109.cfc:49` variable 'rce' has no component ref |
| 1 | `repo → test/tickets/LDEV2309.cfc` | 1 | medium | declares getUrl() | `test/tickets/LDEV6317.cfc:10` variable 'repo' has no component ref |
| 1 | `request.testfilter` | 0 | none |  | `test/run-tests.cfm:282` variable 'request.testFilter' has no component ref |
| 1 | `request.testlabels` | 0 | none |  | `test/run-tests.cfm:307` variable 'request.testLabels' has no component ref |
| 1 | `result → test/tickets/LDEV1962/component1.cfc` | 1 | medium | declares getmessage() | `test/tickets/_LDEV1501.cfc:7` variable 'result' has no component ref |
| 1 | `result.cookies` | 0 | none |  | `test/tickets/LDEV4756.cfc:202` variable 'result.cookies' has no component ref |
| 1 | `resultvar` | 0 | none |  | `test/tickets/LDEV4425/test4425.cfm:12` variable 'resultVar' has no component ref |
| 1 | `s → test/jira/Jira2620.cfc` | 1 | medium | declares createFile() | `test/general/Resources.cfc:337` variable 's' has no component ref |
| 1 | `scopecontext` | 0 | none |  | `test/tickets/LDEV5930/evictSession.cfm:13` variable 'scopeContext' has no component ref |
| 1 | `server.ldev3478[]` | 0 | none |  | `test/tickets/LDEV3478.cfc:143` variable 'server.LDEV3478[]' has no component ref |
| 1 | `settings.sessioncommitinterval` | 0 | none |  | `test/tickets/LDEV6331/settings.cfm:5` variable 'settings.sessionCommitInterval' has no component ref |
| 1 | `settings.sessiontimeout` | 0 | none |  | `test/tickets/LDEV6331/settings.cfm:6` variable 'settings.sessionTimeout' has no component ref |
| 1 | `settings.xmlfeatures` | 0 | none |  | `test/tickets/LDEV4348/LDEV4348.cfm:3` variable 'settings.xmlFeatures' has no component ref |
| 1 | `settingsafter.templatecharset → test/tickets/LDEV1818/Sample.cfc` | 4 | low | declares name() (4 candidates) | `test/tickets/LDEV6085.cfc:116` variable 'settingsAfter.templateCharset' has no component ref |
| 1 | `settingsbefore.templatecharset → test/tickets/LDEV1818/Sample.cfc` | 4 | low | declares name() (4 candidates) | `test/tickets/LDEV6085.cfc:91` variable 'settingsBefore.templateCharset' has no component ref |
| 1 | `smtpserver` | 0 | none |  | `test/tags/_Mail.cfc:56` variable 'smtpServer' has no component ref |
| 1 | `src.dumpwriters` | 0 | none |  | `test/tickets/LDEV5758.cfc:259` variable 'src.dumpWriters' has no component ref |
| 1 | `ss` | 0 | none |  | `test/general/Resources.cfc:339` variable 'ss' has no component ref |
| 1 | `sss → test/jira/Jira2620.cfc` | 1 | medium | declares createFile() | `test/general/Resources.cfc:341` variable 'sss' has no component ref |
| 1 | `st.template` | 0 | none |  | `test/_testRunner.cfc:247` variable 'st.template' has no component ref |
| 1 | `static → test/tickets/LDEV1227/comp1.cfc` | 1 | medium | declares myUCase() | `test/tickets/LDEV1227/comp2.cfc:26` variable 'static' has no component ref |
| 1 | `static → test/tickets/LDEV1227/comp2.cfc` | 1 | medium | declares myUCase2() | `test/tickets/LDEV1227/comp2.cfc:23` variable 'static' has no component ref |
| 1 | `static → test/tickets/LDEV1431/test.cfc` | 1 | medium | declares testStatic() | `test/tickets/LDEV1431/test.cfc:7` variable 'static' has no component ref |
| 1 | `str` | 0 | none |  | `test/tickets/LDEV3167.cfc:14` variable 'str' has no component ref |
| 1 | `stream` | 0 | none |  | `test/run-tests.cfm:23` variable 'stream' has no component ref |
| 1 | `subarr` | 0 | none |  | `test/tickets/LDEV3958.cfc:35` variable 'subArr' has no component ref |
| 1 | `t → test/orm/many2many/model/Bookmark.cfc` | 96 | low | declares getId() (96 candidates) | `test/orm/many2many/index.cfm:72` variable 't' has no component ref |
| 1 | `tb → test/tickets/LDEV5601.cfc` | 4 | medium | declares getVersion() | `test/run-tests.cfm:454` variable 'tb' has no component ref |
| 1 | `test` | 0 | none |  | `test/tickets/LDEV3070/LDEV3070.cfm:8` variable 'test' has no component ref |
| 1 | `test → test/tickets/LDEV2390/Test.cfc` | 12 | medium | declares foo(); named like the receiver 'test' (10 candidates) | `test/tickets/LDEV1487.cfc:34` variable 'test' has no component ref |
| 1 | `testquery` | 0 | none |  | `test/tickets/LDEV3640.cfc:85` variable 'testquery' has no component ref |
| 1 | `testquery2` | 0 | none |  | `test/tickets/LDEV3640.cfc:96` variable 'testquery2' has no component ref |
| 1 | `testresults` | 0 | none |  | `test/_testRunner.cfc:111` variable 'testResults' has no component ref |
| 1 | `today` | 0 | none |  | `test/tickets/Issue0003.cfc:32` variable 'today' has no component ref |
| 1 | `variables.a.b.c → test/general/InlineComponent.cfc` | 2 | low | declares d() (2 candidates) | `test/general/FunctionListener.cfc:89` variable 'variables.a.b.c' has no component ref |
| 1 | `wb` | 0 | none |  | `test/tickets/_LDEV1501.cfc:6` variable 'wb' has no component ref |
| 1 | `white2` | 0 | none |  | `test/tickets/LDEV5640.cfc:16` variable 'white2' has no component ref |
| 1 | `ws → test/tickets/LDEV3629/test.cfc` | 1 | medium | declares getCustomer() | `test/tickets/LDEV3629.cfc:8` variable 'ws' has no component ref |

<details><summary>Groups with several candidates</summary>

- `driver → core/src/main/java/resource/context/admin/mailservers/GMX.cfc` — 14 finding(s), 8 candidate(s):
  - low `core/src/main/java/resource/context/admin/mailservers/GMX.cfc` — declares getShortName(), getLabel(), getHost(), getPort(), useSSL(), useTLS()
  - low `core/src/main/java/resource/context/admin/mailservers/GMail.cfc` — declares getShortName(), getLabel(), getHost(), getPort(), useSSL(), useTLS()
  - low `core/src/main/java/resource/context/admin/mailservers/MailCom.cfc` — declares getShortName(), getLabel(), getHost(), getPort(), useSSL(), useTLS()
  - low `core/src/main/java/resource/context/admin/mailservers/MailServer.cfc` — declares getShortName(), getLabel(), getHost(), getPort(), useSSL(), useTLS()
  - low `core/src/main/java/resource/context/admin/mailservers/Other.cfc` — declares getShortName(), getLabel(), getHost(), getPort(), useSSL(), useTLS()
  - low `core/src/main/java/resource/context/admin/mailservers/Outlook.cfc` — declares getShortName(), getLabel(), getHost(), getPort(), useSSL(), useTLS()
  - low `core/src/main/java/resource/context/admin/mailservers/Yahoo.cfc` — declares getShortName(), getLabel(), getHost(), getPort(), useSSL(), useTLS()
  - low `core/src/main/java/resource/context/admin/mailservers/iCloud.cfc` — declares getShortName(), getLabel(), getHost(), getPort(), useSSL(), useTLS()
- `application.objects.utils → core/src/main/cfml/context/doc/DocUtils.cfc` — 9 finding(s), 3 candidate(s):
  - low `core/src/main/cfml/context/doc/DocUtils.cfc` — declares getAllFunctions()
  - low `core/src/main/java/resource/context/admin/debug/Modern.cfc` — declares getAllFunctions()
  - low `core/src/main/java/resource/context/admin/info/Info.cfc` — declares getAllFunctions()
- `p → test/tickets/LDEV3102.cfc` — 7 finding(s), 10 candidate(s):
  - low `test/tickets/LDEV3102.cfc` — declares getDatasource()
  - low `test/tickets/LDEV3127.cfc` — declares getDatasource()
  - low `test/tickets/LDEV3659.cfc` — declares getDatasource()
  - low `test/tickets/LDEV4485.cfc` — declares getDatasource()
  - low `test/tickets/LDEV3487/Application.cfc` — declares getDatasource()
  - low `test/tickets/LDEV3860/Application.cfc` — declares getDatasource()
  - low `test/tickets/LDEV3907/Application.cfc` — declares getDatasource()
  - low `test/datasource/MySQL.cfc` — declares getDatasource()
  - … 2 more
- `rtn → test/jira/Jira1460.cfc` — 6 finding(s), 5 candidate(s):
  - low `test/jira/Jira1460.cfc` — declares a(), b()
  - low `test/jira/Jira2726.cfc` — declares a(), b()
  - low `test/functions/_SerializeJSON2.cfc` — declares a(), b()
  - low `test/general/InlineComponent.cfc` — declares a(), b()
  - low `test/general/subComponent/TestSubScript.cfc` — declares a(), b()
- `drivers[] → core/src/main/java/resource/context/admin/mailservers/GMX.cfc` — 5 finding(s), 8 candidate(s):
  - low `core/src/main/java/resource/context/admin/mailservers/GMX.cfc` — declares getHost(), getPort(), useSSL(), useTLS(), getLabel()
  - low `core/src/main/java/resource/context/admin/mailservers/GMail.cfc` — declares getHost(), getPort(), useSSL(), useTLS(), getLabel()
  - low `core/src/main/java/resource/context/admin/mailservers/MailCom.cfc` — declares getHost(), getPort(), useSSL(), useTLS(), getLabel()
  - low `core/src/main/java/resource/context/admin/mailservers/MailServer.cfc` — declares getHost(), getPort(), useSSL(), useTLS(), getLabel()
  - low `core/src/main/java/resource/context/admin/mailservers/Other.cfc` — declares getHost(), getPort(), useSSL(), useTLS(), getLabel()
  - low `core/src/main/java/resource/context/admin/mailservers/Outlook.cfc` — declares getHost(), getPort(), useSSL(), useTLS(), getLabel()
  - low `core/src/main/java/resource/context/admin/mailservers/Yahoo.cfc` — declares getHost(), getPort(), useSSL(), useTLS(), getLabel()
  - low `core/src/main/java/resource/context/admin/mailservers/iCloud.cfc` — declares getHost(), getPort(), useSSL(), useTLS(), getLabel()
- `driver → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — 4 finding(s), 21 candidate(s):
  - low `core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/aidriver/Gemini.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/aidriver/OpenAI.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/cdriver/MemCache.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/cdriver/RamCache.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/ActiveMQ.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/AsynchronousEvents.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/DirectoryWatcher.cfc` — declares getLabel(), getClass()
  - … 13 more
- `drivers[] → core/src/main/java/resource/context/admin/debug/Classic.cfc` — 4 finding(s), 4 candidate(s):
  - low `core/src/main/java/resource/context/admin/debug/Classic.cfc` — declares getDescription(), getId(), getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Comment.cfc` — declares getDescription(), getId(), getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Modern.cfc` — declares getDescription(), getId(), getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Simple.cfc` — declares getDescription(), getId(), getLabel()
- `httpsession → test/tickets/LDEV0078/Test.cfc` — 4 finding(s), 96 candidate(s):
  - low `test/tickets/LDEV0078/Test.cfc` — declares getId()
  - low `test/tickets/LDEV0087/Entity.cfc` — declares getId()
  - low `test/tickets/LDEV0096/Entity.cfc` — declares getId()
  - low `test/tickets/LDEV0374/users.cfc` — declares getId()
  - low `test/tickets/LDEV0421/test.cfc` — declares getId()
  - low `test/tickets/LDEV0423/BasketEntity.cfc` — declares getId()
  - low `test/tickets/LDEV0423/FruitEntity.cfc` — declares getId()
  - low `test/tickets/LDEV0490/haspersistent.cfc` — declares getId()
  - … 88 more
- `notexa.b.c → test/general/InlineComponent.cfc` — 4 finding(s), 2 candidate(s):
  - low `test/general/InlineComponent.cfc` — declares d()
  - low `test/general/subComponent/TestSubScript.cfc` — declares d()
- `tz → test/jira/Jira2275/orm/FieldLink.cfc` — 4 finding(s), 14 candidate(s):
  - low `test/jira/Jira2275/orm/FieldLink.cfc` — declares getDisplayName()
  - low `core/src/main/java/resource/context/admin/aidriver/Field.cfc` — declares getDisplayName()
  - low `core/src/main/java/resource/context/admin/aidriver/Group.cfc` — declares getDisplayName()
  - low `core/src/main/java/resource/context/admin/cdriver/Field.cfc` — declares getDisplayName()
  - low `core/src/main/java/resource/context/admin/cdriver/Group.cfc` — declares getDisplayName()
  - low `core/src/main/java/resource/context/admin/debug/Field.cfc` — declares getDisplayName()
  - low `core/src/main/java/resource/context/admin/debug/Group.cfc` — declares getDisplayName()
  - low `core/src/main/java/resource/context/admin/gdriver/Field.cfc` — declares getDisplayName()
  - … 6 more
- `a → test/general/InlineComponent.cfc` — 3 finding(s), 5 candidate(s):
  - low `test/general/InlineComponent.cfc` — declares b()
  - low `test/general/subComponent/TestSubScript.cfc` — declares b()
  - low `test/functions/_SerializeJSON2.cfc` — declares b()
  - low `test/jira/Jira1460.cfc` — declares b()
  - low `test/jira/Jira2726.cfc` — declares b()
- `a.b.c → test/general/InlineComponent.cfc` — 3 finding(s), 2 candidate(s):
  - low `test/general/InlineComponent.cfc` — declares d()
  - low `test/general/subComponent/TestSubScript.cfc` — declares d()
- `appender → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — 3 finding(s), 21 candidate(s):
  - low `core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/aidriver/Gemini.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/aidriver/OpenAI.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/cdriver/MemCache.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/cdriver/RamCache.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/ActiveMQ.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/AsynchronousEvents.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/DirectoryWatcher.cfc` — declares getLabel(), getClass()
  - … 13 more
- `driver → core/src/main/java/resource/context/admin/debug/Classic.cfc` — 3 finding(s), 5 candidate(s):
  - low `core/src/main/java/resource/context/admin/debug/Classic.cfc` — declares output()
  - low `core/src/main/java/resource/context/admin/debug/Comment.cfc` — declares output()
  - low `core/src/main/java/resource/context/admin/debug/Debug.cfc` — declares output()
  - low `core/src/main/java/resource/context/admin/debug/Modern.cfc` — declares output()
  - low `core/src/main/java/resource/context/admin/debug/Simple.cfc` — declares output()
- `layout → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — 3 finding(s), 21 candidate(s):
  - low `core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/aidriver/Gemini.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/aidriver/OpenAI.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/cdriver/MemCache.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/cdriver/RamCache.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/ActiveMQ.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/AsynchronousEvents.cfc` — declares getLabel(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/DirectoryWatcher.cfc` — declares getLabel(), getClass()
  - … 13 more
- `_defaultdriver → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — 2 finding(s), 40 candidate(s):
  - low `core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/aidriver/Gemini.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/aidriver/OpenAI.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/cdriver/MemCache.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/cdriver/RamCache.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Classic.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Comment.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Modern.cfc` — declares getLabel()
  - … 32 more
- `arguments.debugtemplate → core/src/main/java/resource/context/admin/debug/Classic.cfc` — 2 finding(s), 5 candidate(s):
  - low `core/src/main/java/resource/context/admin/debug/Classic.cfc` — declares output()
  - low `core/src/main/java/resource/context/admin/debug/Comment.cfc` — declares output()
  - low `core/src/main/java/resource/context/admin/debug/Debug.cfc` — declares output()
  - low `core/src/main/java/resource/context/admin/debug/Modern.cfc` — declares output()
  - low `core/src/main/java/resource/context/admin/debug/Simple.cfc` — declares output()
- `code → test/orm/clearSession/Code.cfc` — 2 finding(s), 7 candidate(s):
  - low `test/orm/clearSession/Code.cfc` — declares getId(), getCode(); named like the receiver 'code'
  - low `test/orm/events/Code.cfc` — declares getId(), getCode(); named like the receiver 'code'
  - low `test/orm/many2one/Code.cfc` — declares getId(), getCode(); named like the receiver 'code'
  - low `test/orm/simple/Code.cfc` — declares getId(), getCode(); named like the receiver 'code'
  - low `test/tickets/LDEV0881/Code.cfc` — declares getId(), getCode(); named like the receiver 'code'
  - low `test/orm/many2one/Ref.cfc` — declares getId(), getCode()
  - low `test/tickets/LDEV0881/Ref.cfc` — declares getId(), getCode()
- `code → test/tickets/LDEV0881/Code.cfc` — 2 finding(s), 7 candidate(s):
  - low `test/tickets/LDEV0881/Code.cfc` — declares getId(), getCode(); named like the receiver 'code'
  - low `test/orm/clearSession/Code.cfc` — declares getId(), getCode(); named like the receiver 'code'
  - low `test/orm/events/Code.cfc` — declares getId(), getCode(); named like the receiver 'code'
  - low `test/orm/many2one/Code.cfc` — declares getId(), getCode(); named like the receiver 'code'
  - low `test/orm/simple/Code.cfc` — declares getId(), getCode(); named like the receiver 'code'
  - low `test/tickets/LDEV0881/Ref.cfc` — declares getId(), getCode()
  - low `test/orm/many2one/Ref.cfc` — declares getId(), getCode()
- `odnsdomain → test/tickets/LDEV0078/Test.cfc` — 2 finding(s), 96 candidate(s):
  - low `test/tickets/LDEV0078/Test.cfc` — declares getId()
  - low `test/tickets/LDEV0087/Entity.cfc` — declares getId()
  - low `test/tickets/LDEV0096/Entity.cfc` — declares getId()
  - low `test/tickets/LDEV0374/users.cfc` — declares getId()
  - low `test/tickets/LDEV0421/test.cfc` — declares getId()
  - low `test/tickets/LDEV0423/BasketEntity.cfc` — declares getId()
  - low `test/tickets/LDEV0423/FruitEntity.cfc` — declares getId()
  - low `test/tickets/LDEV0490/haspersistent.cfc` — declares getId()
  - … 88 more
- `qry.driver → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — 2 finding(s), 40 candidate(s):
  - low `core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/aidriver/Gemini.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/aidriver/OpenAI.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/cdriver/MemCache.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/cdriver/RamCache.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Classic.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Comment.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Modern.cfc` — declares getLabel()
  - … 32 more
- `ref → test/orm/many2one/Ref.cfc` — 2 finding(s), 7 candidate(s):
  - low `test/orm/many2one/Ref.cfc` — declares getId(), getCode(); named like the receiver 'ref'
  - low `test/tickets/LDEV0881/Ref.cfc` — declares getId(), getCode(); named like the receiver 'ref'
  - low `test/orm/clearSession/Code.cfc` — declares getId(), getCode()
  - low `test/orm/events/Code.cfc` — declares getId(), getCode()
  - low `test/orm/many2one/Code.cfc` — declares getId(), getCode()
  - low `test/orm/simple/Code.cfc` — declares getId(), getCode()
  - low `test/tickets/LDEV0881/Code.cfc` — declares getId(), getCode()
- `ref → test/tickets/LDEV0881/Ref.cfc` — 2 finding(s), 7 candidate(s):
  - low `test/tickets/LDEV0881/Ref.cfc` — declares getId(), getCode(); named like the receiver 'ref'
  - low `test/orm/many2one/Ref.cfc` — declares getId(), getCode(); named like the receiver 'ref'
  - low `test/tickets/LDEV0881/Code.cfc` — declares getId(), getCode()
  - low `test/orm/clearSession/Code.cfc` — declares getId(), getCode()
  - low `test/orm/events/Code.cfc` — declares getId(), getCode()
  - low `test/orm/many2one/Code.cfc` — declares getId(), getCode()
  - low `test/orm/simple/Code.cfc` — declares getId(), getCode()
- `val → test/tickets/LDEV0966/orm/tblitem.cfc` — 2 finding(s), 11 candidate(s):
  - low `test/tickets/LDEV0966/orm/tblitem.cfc` — declares getType()
  - low `test/jira/Jira2275/orm/FieldLink.cfc` — declares getType()
  - low `core/src/main/java/resource/context/admin/aidriver/Field.cfc` — declares getType()
  - low `core/src/main/java/resource/context/admin/cdriver/Field.cfc` — declares getType()
  - low `core/src/main/java/resource/context/admin/dbdriver/Driver.cfc` — declares getType()
  - low `core/src/main/java/resource/context/admin/debug/Field.cfc` — declares getType()
  - low `core/src/main/java/resource/context/admin/gdriver/Field.cfc` — declares getType()
  - low `core/src/main/java/resource/context/admin/dbdriver/types/Driver.cfc` — declares getType()
  - … 3 more
- `a.b → test/general/InlineComponent.cfc` — 1 finding(s), 3 candidate(s):
  - low `test/general/InlineComponent.cfc` — declares c()
  - low `test/general/subComponent/TestSubScript.cfc` — declares c()
  - low `test/jira/Jira2726.cfc` — declares c()
- `adm → test/tickets/LDEV6066.cfc` — 1 finding(s), 2 candidate(s):
  - low `test/tickets/LDEV6066.cfc` — declares removeDatasource()
  - low `core/src/main/java/resource/component/org/lucee/cfml/Administrator.cfc` — declares removeDatasource()
- `arguments.a.b.c → test/general/InlineComponent.cfc` — 1 finding(s), 2 candidate(s):
  - low `test/general/InlineComponent.cfc` — declares d()
  - low `test/general/subComponent/TestSubScript.cfc` — declares d()
- `arguments.l → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — 1 finding(s), 40 candidate(s):
  - low `core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/aidriver/Gemini.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/aidriver/OpenAI.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/cdriver/MemCache.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/cdriver/RamCache.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Classic.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Comment.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Modern.cfc` — declares getLabel()
  - … 32 more
- `arguments.r → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — 1 finding(s), 40 candidate(s):
  - low `core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/aidriver/Gemini.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/aidriver/OpenAI.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/cdriver/MemCache.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/cdriver/RamCache.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Classic.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Comment.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/debug/Modern.cfc` — declares getLabel()
  - … 32 more
- `cfcc → test/jira/Jira0713/Test.cfc` — 1 finding(s), 56 candidate(s):
  - low `test/jira/Jira0713/Test.cfc` — declares getName(), setName()
  - low `test/jira/Jira2275/orm/Field.cfc` — declares getName(), setName()
  - low `test/functions/ORMExecuteQuery/Person.cfc` — declares getName(), setName()
  - low `test/orm/transRollback/Person.cfc` — declares getName(), setName()
  - low `test/orm/transSave/Person.cfc` — declares getName(), setName()
  - low `test/orm/transSaveExCommit/Person.cfc` — declares getName(), setName()
  - low `test/orm/transSaveFlush/Person.cfc` — declares getName(), setName()
  - low `test/orm/transSavepoint/Person.cfc` — declares getName(), setName()
  - … 48 more
- `config → test/tickets/LDEV5567/LDEV5660.cfc` — 1 finding(s), 2 candidate(s):
  - low `test/tickets/LDEV5567/LDEV5660.cfc` — declares getMappings()
  - low `core/src/main/java/resource/component/org/lucee/cfml/Administrator.cfc` — declares getMappings()
- `d → core/src/main/java/resource/context/admin/gdriver/ActiveMQ.cfc` — 1 finding(s), 6 candidate(s):
  - low `core/src/main/java/resource/context/admin/gdriver/ActiveMQ.cfc` — declares getCFCPath(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/AsynchronousEvents.cfc` — declares getCFCPath(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/DirectoryWatcher.cfc` — declares getCFCPath(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/JMS.cfc` — declares getCFCPath(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/MailWatcher.cfc` — declares getCFCPath(), getClass()
  - low `core/src/main/java/resource/context/admin/gdriver/TaskGatewayDriver.cfc` — declares getCFCPath(), getClass()
- `drivers[] → core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — 1 finding(s), 48 candidate(s):
  - low `core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — declares getDescription()
  - low `core/src/main/java/resource/context/admin/aidriver/Field.cfc` — declares getDescription()
  - low `core/src/main/java/resource/context/admin/aidriver/Gemini.cfc` — declares getDescription()
  - low `core/src/main/java/resource/context/admin/aidriver/Group.cfc` — declares getDescription()
  - low `core/src/main/java/resource/context/admin/aidriver/OpenAI.cfc` — declares getDescription()
  - low `core/src/main/java/resource/context/admin/cdriver/Field.cfc` — declares getDescription()
  - low `core/src/main/java/resource/context/admin/cdriver/Group.cfc` — declares getDescription()
  - low `core/src/main/java/resource/context/admin/cdriver/MemCache.cfc` — declares getDescription()
  - … 40 more
- `kind → test/tickets/LDEV1818/Sample.cfc` — 1 finding(s), 4 candidate(s):
  - low `test/tickets/LDEV1818/Sample.cfc` — declares name()
  - low `test/tickets/LDEV1818/test.cfc` — declares name()
  - low `test/tickets/LDEV0279/success/Irate.cfc` — declares name()
  - low `test/tickets/LDEV0279/success/Rate.cfc` — declares name()
- `local.a.b.c → test/general/InlineComponent.cfc` — 1 finding(s), 2 candidate(s):
  - low `test/general/InlineComponent.cfc` — declares d()
  - low `test/general/subComponent/TestSubScript.cfc` — declares d()
- `ml → test/orm/many2many/model/Bookmark.cfc` — 1 finding(s), 96 candidate(s):
  - low `test/orm/many2many/model/Bookmark.cfc` — declares getId()
  - low `test/orm/many2many/model/Tag.cfc` — declares getId()
  - low `test/orm/many2many/model/module.cfc` — declares getId()
  - low `test/orm/many2many/model/moduleLang.cfc` — declares getId()
  - low `test/orm/clearSession/Code.cfc` — declares getId()
  - low `test/orm/events/Code.cfc` — declares getId()
  - low `test/orm/many2one/Code.cfc` — declares getId()
  - low `test/orm/many2one/Ref.cfc` — declares getId()
  - … 88 more
- `newclass → core/src/main/java/resource/context/admin/dbdriver/Driver.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/src/main/java/resource/context/admin/dbdriver/Driver.cfc` — declares init(), getValue()
  - low `core/src/main/java/resource/context/admin/dbdriver/types/Driver.cfc` — declares init(), getValue()
- `obj[] → test/tickets/LDEV1741/App1/model/Foo.cfc` — 1 finding(s), 40 candidate(s):
  - low `test/tickets/LDEV1741/App1/model/Foo.cfc` — declares getLabel()
  - low `test/tickets/LDEV1741/App2/model/Foo.cfc` — declares getLabel()
  - low `test/tickets/LDEV1937/msSql.cfc` — declares getLabel()
  - low `test/tickets/LDEV1937/mySql.cfc` — declares getLabel()
  - low `test/tickets/LDEV1641/specs/CardProcessorType.cfc` — declares getLabel()
  - low `test/tickets/LDEV1659/model/Foo.cfc` — declares getLabel()
  - low `test/tickets/LDEV1984/App1/model/Foo.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — declares getLabel()
  - … 32 more
- `obj[] → test/tickets/LDEV1741/App2/model/Foo.cfc` — 1 finding(s), 40 candidate(s):
  - low `test/tickets/LDEV1741/App2/model/Foo.cfc` — declares getLabel()
  - low `test/tickets/LDEV1741/App1/model/Foo.cfc` — declares getLabel()
  - low `test/tickets/LDEV1937/msSql.cfc` — declares getLabel()
  - low `test/tickets/LDEV1937/mySql.cfc` — declares getLabel()
  - low `test/tickets/LDEV1641/specs/CardProcessorType.cfc` — declares getLabel()
  - low `test/tickets/LDEV1659/model/Foo.cfc` — declares getLabel()
  - low `test/tickets/LDEV1984/App1/model/Foo.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — declares getLabel()
  - … 32 more
- `obj[] → test/tickets/LDEV1984/App1/model/Foo.cfc` — 1 finding(s), 40 candidate(s):
  - low `test/tickets/LDEV1984/App1/model/Foo.cfc` — declares getLabel()
  - low `test/tickets/LDEV1937/msSql.cfc` — declares getLabel()
  - low `test/tickets/LDEV1937/mySql.cfc` — declares getLabel()
  - low `test/tickets/LDEV1641/specs/CardProcessorType.cfc` — declares getLabel()
  - low `test/tickets/LDEV1659/model/Foo.cfc` — declares getLabel()
  - low `test/tickets/LDEV1741/App1/model/Foo.cfc` — declares getLabel()
  - low `test/tickets/LDEV1741/App2/model/Foo.cfc` — declares getLabel()
  - low `core/src/main/java/resource/context/admin/aidriver/Claude.cfc` — declares getLabel()
  - … 32 more
- `proxiedobject → test/tickets/LDEV-4676.cfc` — 1 finding(s), 1476 candidate(s):
  - low `test/tickets/LDEV-4676.cfc` — declares run()
  - low `test/tickets/LDEV-4680.cfc` — declares run()
  - low `test/tickets/LDEV-4695.cfc` — declares run()
  - low `test/tickets/LDEV-4795.cfc` — declares run()
  - low `test/tickets/LDEV-4796.cfc` — declares run()
  - low `test/tickets/LDEV-5093.cfc` — declares run()
  - low `test/tickets/LDEV0028.cfc` — declares run()
  - low `test/tickets/LDEV0066.cfc` — declares run()
  - … 1468 more
- `proxy → test/functions/CreateDynamicProxy/javaSetting/Test.cfc` — 1 finding(s), 2 candidate(s):
  - low `test/functions/CreateDynamicProxy/javaSetting/Test.cfc` — declares hello()
  - low `test/jira/Jira3096/Test.cfc` — declares hello()
- `proxy → test/jira/Jira3096/Test.cfc` — 1 finding(s), 2 candidate(s):
  - low `test/jira/Jira3096/Test.cfc` — declares hello()
  - low `test/functions/CreateDynamicProxy/javaSetting/Test.cfc` — declares hello()
- `r → test/tickets/LDEV6138/TestEntity.cfc` — 1 finding(s), 56 candidate(s):
  - low `test/tickets/LDEV6138/TestEntity.cfc` — declares getName(), setName()
  - low `test/tickets/LDEV0078/Test.cfc` — declares getName(), setName()
  - low `test/tickets/LDEV0423/Basket.cfc` — declares getName(), setName()
  - low `test/tickets/LDEV0423/BasketEntity.cfc` — declares getName(), setName()
  - low `test/tickets/LDEV0423/Fruit.cfc` — declares getName(), setName()
  - low `test/tickets/LDEV0423/FruitEntity.cfc` — declares getName(), setName()
  - low `test/tickets/LDEV0921/RandomEntity.cfc` — declares getName(), setName()
  - low `test/tickets/LDEV1214/RandomEntity.cfc` — declares getName(), setName()
  - … 48 more
- `rce → core/src/main/java/resource/context/admin/dbdriver/Driver.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/src/main/java/resource/context/admin/dbdriver/Driver.cfc` — declares getValue()
  - low `core/src/main/java/resource/context/admin/dbdriver/types/Driver.cfc` — declares getValue()
- `settingsafter.templatecharset → test/tickets/LDEV1818/Sample.cfc` — 1 finding(s), 4 candidate(s):
  - low `test/tickets/LDEV1818/Sample.cfc` — declares name()
  - low `test/tickets/LDEV1818/test.cfc` — declares name()
  - low `test/tickets/LDEV0279/success/Irate.cfc` — declares name()
  - low `test/tickets/LDEV0279/success/Rate.cfc` — declares name()
- `settingsbefore.templatecharset → test/tickets/LDEV1818/Sample.cfc` — 1 finding(s), 4 candidate(s):
  - low `test/tickets/LDEV1818/Sample.cfc` — declares name()
  - low `test/tickets/LDEV1818/test.cfc` — declares name()
  - low `test/tickets/LDEV0279/success/Irate.cfc` — declares name()
  - low `test/tickets/LDEV0279/success/Rate.cfc` — declares name()
- `t → test/orm/many2many/model/Bookmark.cfc` — 1 finding(s), 96 candidate(s):
  - low `test/orm/many2many/model/Bookmark.cfc` — declares getId()
  - low `test/orm/many2many/model/Tag.cfc` — declares getId()
  - low `test/orm/many2many/model/module.cfc` — declares getId()
  - low `test/orm/many2many/model/moduleLang.cfc` — declares getId()
  - low `test/orm/clearSession/Code.cfc` — declares getId()
  - low `test/orm/events/Code.cfc` — declares getId()
  - low `test/orm/many2one/Code.cfc` — declares getId()
  - low `test/orm/many2one/Ref.cfc` — declares getId()
  - … 88 more
- `test → test/tickets/LDEV2390/Test.cfc` — 1 finding(s), 10 candidate(s):
  - medium `test/tickets/LDEV2390/Test.cfc` — declares foo(); named like the receiver 'test'
  - low `test/tickets/LDEV3714.cfc` — declares foo()
  - low `test/tickets/LDEV0240/comp2.cfc` — declares foo()
  - low `test/tickets/LDEV0600/comp1.cfc` — declares foo()
  - low `test/tickets/LDEV0600/comp2.cfc` — declares foo()
  - low `test/tickets/LDEV1059/content.cfc` — declares foo()
  - low `test/tickets/LDEV1221/static.cfc` — declares foo()
  - low `test/tickets/LDEV3184/LDEV3184.cfc` — declares foo()
  - … 2 more
- `variables.a.b.c → test/general/InlineComponent.cfc` — 1 finding(s), 2 candidate(s):
  - low `test/general/InlineComponent.cfc` — declares d()
  - low `test/general/subComponent/TestSubScript.cfc` — declares d()

</details>

## Return types — a call chained on a method that declares no component

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 10 | `method 'exe' has no component return type` | 0 | none |  | `test/tickets/LDEV0907.cfc:34` method 'exe' has no component return type (chain to 'isCached') |
| 2 | `method '_getAntiSamy' has no component return type` | 0 | none |  | `test/tickets/LDEV5085/AntiSamyService.cfc:19` method '_getAntiSamy' has no component return type (chain to 'scan') |
| 2 | `method 'func' has no component return type → test/general/InlineComponent.cfc` | 6 | low | declares b(), c() (3 candidates) | `test/general/InlineComponent.cfc:65` method 'func' has no component return type (chain to 'b') |
| 2 | `method 'send' in http has no component return type → core/src/main/java/resource/component/org/lucee/cfml/Result.cfc` | 1 | medium | declares getPrefix() | `test/tickets/_LDEV1454.cfc:21` method 'send' in http has no component return type (chain to 'getPrefix') |
| 1 | `method 'send' in Http has no component return type → core/src/main/java/resource/component/org/lucee/cfml/Result.cfc` | 1 | medium | declares getPrefix() | `test/tickets/_LDEV1976.cfc:6` method 'send' in Http has no component return type (chain to 'getPrefix') |

<details><summary>Groups with several candidates</summary>

- `method 'func' has no component return type → test/general/InlineComponent.cfc` — 2 finding(s), 3 candidate(s):
  - low `test/general/InlineComponent.cfc` — declares b(), c()
  - low `test/general/subComponent/TestSubScript.cfc` — declares b(), c()
  - low `test/jira/Jira2726.cfc` — declares b(), c()

</details>

## Method definitions — a method not found where it was looked for

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 38 | `toversionsortable` | 2 | low | declares toVersionSortable (2 candidates) | `test/tickets/LDEV6069.cfc:38` not found in extends chain |
| 33 | `writeln` | 0 | none |  | `test/tickets/LDEV6377_regression.cfm:5` no qualifier, not in file |
| 28 | `getcolumn` | 0 | none |  | `test/tickets/LDEV3640.cfc:50` method 'getColumn' not found on builtin querynew |
| 22 | `hasnewerversion` | 1 | medium | declares hasNewerVersion | `test/tickets/LDEV6068.cfc:110` not found in extends chain |
| 18 | `compareversions` | 1 | medium | declares compareVersions | `test/tickets/LDEV6068.cfc:36` not found in extends chain |
| 11 | `tojson` | 0 | none |  | `test/_testFilter.cfc:108` no qualifier, not in file |
| 9 | `categorizeversions` | 1 | medium | declares categorizeVersions | `test/tickets/LDEV6068.cfc:314` not found in extends chain |
| 9 | `mongodbconnect` | 0 | none |  | `test/_setupTestServices.cfc:320` no qualifier, not in file |
| 7 | `getmyarray` | 1 | medium | declares getMyArray | `test/tickets/LDEV801.cfc:20` method 'getMyArray' not found in component |
| 6 | `isstableversion` | 1 | medium | declares isStableVersion | `test/tickets/LDEV6068.cfc:11` not found in extends chain |
| 6 | `mongodbid` | 0 | none |  | `test/a_debug_build/_MongoDB.cfc:125` not found in extends chain |
| 5 | `getdata` | 13 | low | declares getData; beside the calling file (13 candidates) | `test/tickets/LDEV1221/test1.cfm:6` no qualifier, not in file |
| 5 | `getpara` | 1 | medium | declares getPara; beside the calling file | `test/tickets/LDEV5610.cfc:29` method 'getPara' not found in component |
| 5 | `typeof` | 1 | none |  | `test/tickets/LDEV6377_regression.cfm:17` no qualifier, not in file |
| 4 | `action` | 1 | medium | declares action | `core/src/main/cfml/context/admin/plugin/DDNS/overview.cfm:4` no qualifier, not in file |
| 4 | `getid` | 96 | low | declares getId (96 candidates) | `test/tickets/LDEV801.cfc:57` method 'getId' not found in component |
| 4 | `getluceerocks` | 1 | medium | declares getLuceeRocks | `test/tickets/LDEV801.cfc:60` method 'getLuceeRocks' not found in component |
| 4 | `getred` | 3 | low | declares getRed (3 candidates) | `test/tickets/LDEV0554.cfc:27` not found in extends chain |
| 4 | `luceeaihas` | 0 | none |  | `core/src/main/cfml/context/debug/modern/error.cfm:5` no qualifier, not in file |
| 3 | `cf_dollar` | 0 | none |  | `test/tickets/LDEV3689.cfc:68` not found in extends chain |
| 3 | `elseif` | 0 | none |  | `core/src/main/java/resource/component/org/lucee/cfml/Query.cfc:77` not found in extends chain |
| 3 | `getclassname` | 3 | low | declares getClassName; beside the calling file (3 candidates) | `test/tickets/LDEV6250.cfc:19` method 'getClassName' not found in component |
| 3 | `getupdateformajorversion` | 1 | medium | declares getUpdateForMajorVersion | `test/tickets/LDEV6068.cfc:93` not found in extends chain |
| 3 | `getuser` | 1 | medium | declares getUser | `test/tickets/LDEV5816.cfc:39` method 'getUser' not found in component |
| 3 | `getversion` | 4 | low | declares getVersion; beside the calling file (3 candidates) | `test/tickets/LDEV5601.cfc:48` method 'getVersion' not found in component |
| 3 | `luceeaigetmetadata` | 0 | none |  | `core/src/main/cfml/context/debug/modern/error.cfm:44` no qualifier, not in file |
| 3 | `toosgiversion` | 1 | medium | declares toOSGiVersion | `test/tickets/LDEV6069.cfc:14` not found in extends chain |
| 2 | `a` | 5 | low | declares a; beside the calling file (5 candidates) | `test/general/SafeNavigator.cfc:6` not found in extends chain |
| 2 | `expect` | 1 | none |  | `test/tickets/LDEV4694.cfc:18` no qualifier, not in file |
| 2 | `getmessage` | 1 | medium | declares getMessage | `test/tickets/LDEV1962.cfc:28` method 'getMessage' not found in LDEV1962.component2 |
| 2 | `getmessage2` | 1 | medium | declares getMessage2 | `test/tickets/LDEV1962.cfc:33` method 'getMessage2' not found in LDEV1962.component2 |
| 2 | `getmode` | 0 | none |  | `test/functions/FileOpen.cfc:10` method 'getMode' not found on builtin fileopen |
| 2 | `getresource` | 0 | none |  | `test/functions/DirectoryDelete.cfc:77` method 'getResource' not found on builtin fileopen |
| 2 | `getsize` | 0 | none |  | `test/functions/FileOpen.cfc:57` method 'getSize' not found on builtin fileopen |
| 2 | `getsql` | 0 | none |  | `test/tickets/LDEV3863.cfc:12` method 'getSql' not found on builtin queryexecute |
| 2 | `getstatus` | 1 | medium | declares getStatus | `test/functions/FileOpen.cfc:11` method 'getStatus' not found on builtin fileopen |
| 2 | `gettika` | 3 | low | declares getTika; beside the calling file (3 candidates) | `test/tickets/LDEV5600.cfc:38` method 'getTika' not found in component |
| 2 | `luceeinquiryaisession` | 0 | none |  | `core/src/main/cfml/context/debug/modern/error.cfm:55` no qualifier, not in file |
| 2 | `mixin` | 0 | none |  | `test/tickets/LDEV3473/test.cfm:7` method 'mixin' not found in User |
| 2 | `notexisting` | 0 | none |  | `test/tickets/LDEV1201.cfc:70` not found in extends chain |
| 2 | `override` | 0 | none |  | `test/jira/Jira3099/Component1.cfc:30` method 'override' not found in component2 |
| 2 | `sp_xmltest_bug` | 0 | none |  | `test/datasource/Oracle.cfc:122` not found in extends chain |
| 2 | `testfunctioninsidetagisland` | 0 | none |  | `test/tickets/LDEV3740.cfc:14` not found in extends chain |
| 2 | `to_char` | 0 | none |  | `test/datasource/Oracle.cfc:127` not found in extends chain |
| 2 | `valueequals` | 4 | low | declares valueEquals; beside the calling file (4 candidates) | `test/functions/DayOfWeekAsString.cfc:35` not found in extends chain |
| 2 | `valueof` | 0 | none |  | `test/general/StaticMembersInvoke.cfc:31` not found in extends chain |
| 2 | `varchar` | 0 | none |  | `test/datasource/MySQL.cfc:75` not found in extends chain |
| 2 | `whodoyoulove` | 1 | medium | declares whoDoYouLove; beside the calling file | `test/tickets/LDEV4642/LDEV4642.cfm:5` no qualifier, not in file |
| 2 | `xmlelement` | 0 | none |  | `test/datasource/Oracle.cfc:127` not found in extends chain |
| 1 | `abc` | 1 | medium | declares abc | `test/tickets/LDEV1487.cfc:11` not found in extends chain |
| 1 | `accessimportinapp1` | 1 | medium | declares accessImportInApp1 | `test/tickets/LDEV3912.cfc:5` not found in extends chain |
| 1 | `arraynew[]` | 0 | none |  | `test/tickets/LDEV4370/LDEV4370.cfm:9` no qualifier, not in file |
| 1 | `authority` | 0 | none |  | `test/tickets/LDEV6084.cfc:17` not found in extends chain |
| 1 | `bytagreference` | 0 | none |  | `test/tickets/LDEV3668.cfc:8` not found in extends chain |
| 1 | `bytagvalue` | 0 | none |  | `test/tickets/LDEV3668.cfc:14` not found in extends chain |
| 1 | `cast` | 0 | none |  | `test/tickets/LDEV4827.cfc:29` not found in extends chain |
| 1 | `cf_anothertag` | 0 | none |  | `test/tickets/LDEV5836/test-customtags.cfm:17` no qualifier, not in file |
| 1 | `cf_mytag` | 0 | none |  | `test/tickets/LDEV3722.cfc:8` not found in extends chain |
| 1 | `cf_redden` | 0 | none |  | `test/tickets/LDEV1276/test.cfm:6` no qualifier, not in file |
| 1 | `closure` | 0 | none |  | `test/jira/Jira2902/AbsAbs.cfc:25` no qualifier, not in file |
| 1 | `constructor` | 2 | low | declares constructor; beside the calling file (2 candidates) | `test/tickets/LDEV0296/child.cfc:10` super used but no extends |
| 1 | `container` | 4 | low | declares container; beside the calling file (4 candidates) | `test/tickets/LDEV0285/App4.cfc:6` no qualifier, not in file |
| 1 | `createid` | 1 | medium | declares createId | `core/src/main/cfml/context/admin/ext.applications.upload.cfm:159` no qualifier, not in file |
| 1 | `current` | 0 | none |  | `test/tickets/LDEV5632.cfc:50` not found in extends chain |
| 1 | `describe` | 2 | medium | declares describe | `test/tickets/LDEV4694.cfc:16` no qualifier, not in file |
| 1 | `elemnew` | 0 | none |  | `test/tickets/LDEV2936.cfc:15` method 'elemNew' not found on builtin xmlnew |
| 1 | `emoji_test` | 0 | none |  | `test/datasource/MySQL.cfc:73` not found in extends chain |
| 1 | `foo` | 12 | low | declares foo; beside the calling file (12 candidates) | `test/tickets/LDEV0441.cfc:16` not found in extends chain |
| 1 | `getage` | 4 | low | declares getAge (4 candidates) | `test/tickets/LDEV5816.cfc:41` method 'getUser' not found in component (chain to 'getAge') |
| 1 | `getappclassname` | 1 | medium | declares getAppClassName; beside the calling file | `test/tickets/LDEV6084.cfc:27` method 'getAppClassName' not found in component |
| 1 | `getarr` | 0 | none |  | `test/tickets/LDEV5816.cfc:31` method 'getArr' not found in component |
| 1 | `getbase64string` | 0 | none |  | `test/tickets/LDEV5640.cfc:16` method 'getBase64String' not found on builtin imagenew |
| 1 | `getdatasourcename` | 0 | none |  | `test/tickets/LDEV3070/LDEV3070.cfm:13` method 'getDatasourceName' not found on builtin queryexecute |
| 1 | `getimagebytes` | 0 | none |  | `test/tickets/LDEV1576/test.cfm:4` method 'getImageBytes' not found on builtin imagenew |
| 1 | `getinstance` | 6 | low | declares getInstance; beside the calling file (6 candidates) | `test/tickets/LDEV801.cfc:15` method 'getInstance' not found in component |
| 1 | `getlistener` | 0 | none |  | `core/src/main/cfml/context/gateway/AsynchronousEvents.cfc:66` no qualifier, not in file |
| 1 | `getmessage2x` | 0 | none |  | `test/tickets/LDEV1962/component2.cfc:17` no qualifier, not in file |
| 1 | `getmessagex` | 0 | none |  | `test/tickets/LDEV1962/component2.cfc:9` no qualifier, not in file |
| 1 | `getpddocument` | 1 | medium | declares getPDDocument; beside the calling file | `test/tickets/LDEV5625.cfc:25` method 'getPDDocument' not found in component |
| 1 | `getproviderdata` | 0 | none |  | `core/src/main/cfml/context/admin/ext.applications.list.cfm:79` no qualifier, not in file |
| 1 | `getqry` | 0 | none |  | `test/tickets/LDEV5816.cfc:32` method 'getQry' not found in component |
| 1 | `getst` | 0 | none |  | `test/tickets/LDEV5816.cfc:30` method 'getSt' not found in component |
| 1 | `getstr` | 1 | medium | declares getStr; beside the calling file | `test/tickets/LDEV4156.cfc:15` method 'getStr' not found in Component |
| 1 | `getusername` | 6 | low | declares getUserName (6 candidates) | `test/tickets/LDEV5816.cfc:40` method 'getUser' not found in component (chain to 'getUsername') |
| 1 | `invokestatic` | 0 | none |  | `test/tickets/_LDEV1706.cfc:23` not found in extends chain |
| 1 | `isstring` | 0 | none |  | `test/tickets/LDEV6377_regression.cfm:82` no qualifier, not in file |
| 1 | `it` | 2 | medium | declares it | `test/tickets/LDEV4694.cfc:17` no qualifier, not in file |
| 1 | `listener[]` | 0 | none |  | `core/src/main/cfml/context/gateway/MailWatcher.cfc:56` no qualifier, not in file |
| 1 | `ljklkju` | 0 | none |  | `test/general/Elvis.cfc:60` not found in extends chain |
| 1 | `loadprovidersdata` | 0 | none |  | `core/src/main/cfml/context/admin/ext.applications.upload.cfm:140` no qualifier, not in file |
| 1 | `methodwhichcallsecho` | 1 | medium | declares methodWhichCallsEcho; beside the calling file | `test/tickets/LDEV5792/ldev5792_echo_only.cfm:10` method 'methodWhichCallsEcho' not found in component |
| 1 | `methodwhichcallswritedump` | 2 | low | declares methodWhichCallsWriteDump; beside the calling file (2 candidates) | `test/tickets/LDEV5792/ldev5792_simple.cfm:12` method 'methodWhichCallsWriteDump' not found in component |
| 1 | `myfunc` | 1 | medium | declares myFunc | `test/tickets/LDEV1995/test.cfm:1` no qualifier, not in file |
| 1 | `myfunc2` | 1 | medium | declares myFunc2 | `test/tickets/LDEV1995/test.cfm:1` no qualifier, not in file |
| 1 | `nonexistentmethod` | 0 | none |  | `test/tickets/LDEV6055/Child.cfc:4` not found in parent component |
| 1 | `oncomplete` | 0 | none |  | `test/tickets/LDEV2213/test.cfm:5` no qualifier, not in file |
| 1 | `outer` | 1 | medium | declares outer; beside the calling file | `test/tickets/LDEV5792/ldev5792_nested_call.cfm:17` method 'outer' not found in component |
| 1 | `reportservicefailed` | 1 | medium | declares reportServiceFailed; beside the calling file | `test/run-tests.cfm:617` no qualifier, not in file |
| 1 | `returnsany` | 2 | low | declares returnsAny (2 candidates) | `test/tickets/_LDEV1835.cfc:6` method 'returnsany' not found in LDEV1835.Comp |
| 1 | `rluceemonoblock` | 0 | none |  | `core/src/main/cfml/context/templates/error/error.cfm:252` no qualifier, not in file |
| 1 | `setentityid` | 7 | low | declares setEntityId (7 candidates) | `test/tickets/LDEV0405/index.cfm:25` method 'setEntityId' not found in Comp |
| 1 | `setentitytypeid` | 7 | low | declares setEntityTypeId (7 candidates) | `test/tickets/LDEV0405/index.cfm:26` method 'setEntityTypeId' not found in Comp |
| 1 | `setmyarray` | 1 | medium | declares setMyArray | `test/tickets/LDEV801.cfc:24` method 'setMyArray' not found in component |
| 1 | `setnumid` | 0 | none |  | `test/tickets/LDEV5610.cfc:29` method 'getPara' not found in component (chain to 'setNumID') |
| 1 | `setspacingafter` | 0 | none |  | `test/tickets/LDEV5610.cfc:35` method 'getPara' not found in component (chain to 'setSpacingAfter') |
| 1 | `setspacingbetween` | 0 | none |  | `test/tickets/LDEV5610.cfc:41` method 'getPara' not found in component (chain to 'setSpacingBetween') |
| 1 | `setunitid` | 7 | low | declares setUnitId (7 candidates) | `test/tickets/LDEV0405/index.cfm:24` method 'setUnitId' not found in Comp |
| 1 | `setyear` | 0 | none |  | `test/tickets/LDEV1041.cfc:41` not found in extends chain |
| 1 | `sleeo` | 0 | none |  | `test/tickets/LDEV4967.cfc:11` not found in extends chain |
| 1 | `ss[]` | 0 | none |  | `test/functions/GetComponentStaticScope.cfc:31` not found in extends chain |
| 1 | `susi` | 2 | low | declares susi (2 candidates) | `test/tickets/LDEV4826.cfc:54` not found in extends chain |
| 1 | `testtagattr` | 0 | none |  | `test/tickets/LDEV4008.cfc:7` not found in extends chain |
| 1 | `this[]` | 0 | none |  | `core/src/main/cfml/context/admin/plugin/Plugin.cfc:74` no qualifier, not in file |
| 1 | `xmlserialize` | 0 | none |  | `test/datasource/Oracle.cfc:127` not found in extends chain |

<details><summary>Groups with several candidates</summary>

- `toversionsortable` — 38 finding(s), 2 candidate(s):
  - low `core/src/main/cfml/context/admin/Jira.cfc:933` — declares toVersionSortable
  - low `core/src/main/cfml/context/admin/ext.functions.cfm:645` — declares toVersionSortable
- `getdata` — 5 finding(s), 13 candidate(s):
  - low `test/tickets/LDEV1221/static.cfc:10` — declares getData; beside the calling file
  - low `test/tickets/LDEV0835/A.cfc:7` — declares getData
  - low `test/tickets/LDEV0835/B.cfc:6` — declares getData
  - low `test/tickets/LDEV0835/Base.cfc:7` — declares getData
  - low `test/tickets/LDEV5792/ldev5792_nested_tag.cfm:5` — declares getData
  - low `core/src/main/cfml/context/admin/adminfunctions.cfc:55` — declares getdata
  - low `core/src/main/java/resource/context/admin/aidriver/Field.cfc:51` — declares getData
  - low `core/src/main/java/resource/context/admin/cdriver/Field.cfc:51` — declares getData
  - … 5 more
- `getid` — 4 finding(s), 96 candidate(s):
  - low `test/tickets/LDEV0078/Test.cfc:3` — declares getId
  - low `test/tickets/LDEV0087/Entity.cfc:2` — declares getID
  - low `test/tickets/LDEV0096/Entity.cfc:3` — declares getID
  - low `test/tickets/LDEV0374/users.cfc:2` — declares getId
  - low `test/tickets/LDEV0421/test.cfc:2` — declares getId
  - low `test/tickets/LDEV0423/BasketEntity.cfc:2` — declares getId
  - low `test/tickets/LDEV0423/FruitEntity.cfc:2` — declares getId
  - low `test/tickets/LDEV0490/haspersistent.cfc:3` — declares getID
  - … 88 more
- `getred` — 4 finding(s), 3 candidate(s):
  - low `test/tickets/LDEV0554/Color.cfc:33` — declares getRed
  - low `test/tickets/LDEV0554/Color3.cfc:31` — declares getRed
  - low `test/tickets/LDEV0554/Color4.cfc:28` — declares getRed
- `getclassname` — 3 finding(s), 3 candidate(s):
  - low `test/tickets/LDEV6250.cfc:15` — declares getClassName; beside the calling file
  - low `test/tickets/LDEV6250.cfc:26` — declares getClassName; beside the calling file
  - low `test/tickets/LDEV6251.cfc:11` — declares getClassName; beside the calling file
- `getversion` — 3 finding(s), 3 candidate(s):
  - low `test/tickets/LDEV5601.cfc:43` — declares getVersion; beside the calling file
  - low `test/tickets/LDEV5601.cfc:59` — declares getVersion; beside the calling file
  - low `test/tickets/LDEV5601.cfc:72` — declares getVersion; beside the calling file
- `a` — 2 finding(s), 5 candidate(s):
  - low `test/general/InlineComponent.cfc:29` — declares a; beside the calling file
  - low `test/general/subComponent/TestSubScript.cfc:6` — declares a
  - low `test/functions/_SerializeJSON2.cfc:220` — declares a
  - low `test/jira/Jira1460.cfc:83` — declares a
  - low `test/jira/Jira2726.cfc:34` — declares a
- `gettika` — 2 finding(s), 3 candidate(s):
  - low `test/tickets/LDEV5600.cfc:33` — declares getTika; beside the calling file
  - low `test/tickets/LDEV5708.cfc:39` — declares getTika; beside the calling file
  - low `test/tickets/LDEV5708.cfc:45` — declares getTika; beside the calling file
- `valueequals` — 2 finding(s), 4 candidate(s):
  - low `test/functions/DateConvert.cfc:36` — declares valueEquals; beside the calling file
  - low `test/functions/Decrypt.cfc:34` — declares valueEquals; beside the calling file
  - low `test/functions/DeleteClientVariable.cfc:44` — declares valueEquals; beside the calling file
  - low `test/functions/EncodeForCSS.cfc:35` — declares valueEquals; beside the calling file
- `constructor` — 1 finding(s), 2 candidate(s):
  - low `test/tickets/LDEV0296/child.cfc:9` — declares constructor; beside the calling file
  - low `test/tickets/LDEV0296/parent.cfc:4` — declares constructor; beside the calling file
- `container` — 1 finding(s), 4 candidate(s):
  - low `test/tickets/LDEV0285/App1.cfc:8` — declares container; beside the calling file
  - low `test/tickets/LDEV0285/App2.cfc:8` — declares container; beside the calling file
  - low `test/tickets/LDEV0285/App3.cfc:8` — declares container; beside the calling file
  - low `test/tickets/LDEV0590/test.cfc:6` — declares container
- `foo` — 1 finding(s), 12 candidate(s):
  - low `test/tickets/LDEV3714.cfc:42` — declares foo; beside the calling file
  - low `test/tickets/LDEV0240/comp2.cfc:2` — declares foo
  - low `test/tickets/LDEV0600/comp1.cfc:6` — declares foo
  - low `test/tickets/LDEV0600/comp2.cfc:6` — declares foo
  - low `test/tickets/LDEV1059/content.cfc:3` — declares foo
  - low `test/tickets/LDEV1221/static.cfc:18` — declares foo
  - low `test/tickets/LDEV2390/Test.cfc:6` — declares foo
  - low `test/tickets/LDEV3184/LDEV3184.cfc:3` — declares foo
  - … 4 more
- `getage` — 1 finding(s), 4 candidate(s):
  - low `test/tickets/LDEV5324/student.cfc:4` — declares getAge
  - low `test/tickets/LDEV5816/User.cfc:11` — declares getAge
  - low `test/tickets/LDEV3652_1/objects/objectComponent.cfc:4` — declares getAge
  - low `test/tickets/LDEV3652_2/objects/objectComponent.cfc:4` — declares getAge
- `getinstance` — 1 finding(s), 6 candidate(s):
  - low `test/tickets/LDEV801.cfc:10` — declares getInstance; beside the calling file
  - low `test/tickets/LDEV801.cfc:48` — declares getInstance; beside the calling file
  - low `test/tickets/LDEV801.cfc:74` — declares getInstance; beside the calling file
  - low `test/tickets/LDEV3604/child.cfc:7` — declares getInstance
  - low `test/tickets/LDEV801/ComponentWithComplexDefaultProperty.cfc:3` — declares getInstance
  - low `test/tickets/LDEV801/ComponentWithSimpleDefaultProperties.cfc:5` — declares getInstance
- `getusername` — 1 finding(s), 6 candidate(s):
  - low `test/tickets/LDEV1102/test.cfc:4` — declares getUserName
  - low `test/tickets/LDEV1102/test1.cfc:3` — declares getUserName
  - low `test/tickets/LDEV1102/test2.cfc:4` — declares getUserName
  - low `test/tickets/LDEV1428/ActiveUser.cfc:5` — declares getUserName
  - low `test/tickets/LDEV5816/User.cfc:10` — declares getUsername
  - low `test/tickets/LDEV5930/SessionComponent.cfc:3` — declares getUsername
- `methodwhichcallswritedump` — 1 finding(s), 2 candidate(s):
  - low `test/tickets/LDEV5792/ldev5792_safe_variable.cfm:4` — declares methodWhichCallsWriteDump; beside the calling file
  - low `test/tickets/LDEV5792/ldev5792_simple.cfm:5` — declares methodWhichCallsWriteDump; beside the calling file
- `returnsany` — 1 finding(s), 2 candidate(s):
  - low `test/tickets/LDEV1812/ReturnsString.cfc:3` — declares returnsAny
  - low `test/tickets/LDEV1835/interface.cfc:2` — declares returnsany
- `setentityid` — 1 finding(s), 7 candidate(s):
  - low `test/tickets/LDEV0405/orm/Comp1.cfc:26` — declares setEntityId
  - low `test/tickets/LDEV0405/orm/Comp2.cfc:26` — declares setEntityId
  - low `test/tickets/LDEV0405/orm/Comp3.cfc:26` — declares setEntityId
  - low `test/tickets/LDEV0405/orm/Comp4.cfc:26` — declares setEntityId
  - low `test/jira/Jira3049/orm/MixedComponent.cfc:9` — declares setEntityId
  - low `test/jira/Jira3049/orm/NumericChangedToString.cfc:9` — declares setEntityId
  - low `test/jira/Jira3049/orm/RemovedComposite.cfc:9` — declares setEntityId
- `setentitytypeid` — 1 finding(s), 7 candidate(s):
  - low `test/tickets/LDEV0405/orm/Comp1.cfc:27` — declares setEntityTypeId
  - low `test/tickets/LDEV0405/orm/Comp2.cfc:27` — declares setEntityTypeId
  - low `test/tickets/LDEV0405/orm/Comp3.cfc:27` — declares setEntityTypeId
  - low `test/tickets/LDEV0405/orm/Comp4.cfc:27` — declares setEntityTypeId
  - low `test/jira/Jira3049/orm/MixedComponent.cfc:10` — declares setEntityTypeId
  - low `test/jira/Jira3049/orm/NumericChangedToString.cfc:10` — declares setEntityTypeId
  - low `test/jira/Jira3049/orm/RemovedComposite.cfc:10` — declares setEntityTypeId
- `setunitid` — 1 finding(s), 7 candidate(s):
  - low `test/tickets/LDEV0405/orm/Comp1.cfc:25` — declares setUnitId
  - low `test/tickets/LDEV0405/orm/Comp2.cfc:25` — declares setUnitId
  - low `test/tickets/LDEV0405/orm/Comp3.cfc:25` — declares setUnitId
  - low `test/tickets/LDEV0405/orm/Comp4.cfc:25` — declares setUnitId
  - low `test/jira/Jira3049/orm/MixedComponent.cfc:8` — declares setUnitId
  - low `test/jira/Jira3049/orm/NumericChangedToString.cfc:8` — declares setUnitId
  - low `test/jira/Jira3049/orm/RemovedComposite.cfc:8` — declares setUnitId
- `susi` — 1 finding(s), 2 candidate(s):
  - low `test/general/Struct/invalid3.cfm:2` — declares susi
  - low `test/jira/Jira3099/Component1.cfc:40` — declares susi

</details>

## Object references — a component path that names no file

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 3 | `base component does not resolve; 2 inherited calls not checked` | ? | none |  | `core/src/main/cfml/context/admin/plugin/DDNS/Action.cfc:17` base component does not resolve; 2 inherited calls not checked |
| 3 | `component 'GreenMail' does not exist (calling 'purgeEmailFromAllMailboxes')` | 0 | none |  | `test/tags/MailSpool.cfc:42` component 'GreenMail' does not exist (calling 'purgeEmailFromAllMailboxes') |
| 3 | `component 'LDEV0273.Test' does not exist (calling 'exclaim')` | 0 | low | file named Test.cfc (56 candidates) | `test/tickets/LDEV0273.cfc:23` component 'LDEV0273.Test' does not exist (calling 'exclaim') |
| 3 | `component 'LDEV0273.Test' does not exist (calling 'exclaim2')` | 0 | low | file named Test.cfc (56 candidates) | `test/tickets/LDEV0273.cfc:27` component 'LDEV0273.Test' does not exist (calling 'exclaim2') |
| 3 | `component 'System' does not exist (calling 'setProperty')` | 0 | none |  | `core/src/main/cfml/context/debug/modern/reference.cfm:398` component 'System' does not exist (calling 'setProperty') |
| 3 | `component 'collection' does not exist (calling 'list')` | 0 | none |  | `test/tickets/_LDEV1684.cfc:13` component 'collection' does not exist (calling 'list') |
| 2 | `component 'BundleProvider' does not exist (calling 'getMappings')` | 3 | none |  | `test/tickets/LDEV5567/LDEV5660.cfc:9` component 'BundleProvider' does not exist (calling 'getMappings') |
| 2 | `component 'GreenMail' does not exist (calling 'getReceivedMessages')` | 0 | none |  | `test/tags/MailSpool.cfc:103` component 'GreenMail' does not exist (calling 'getReceivedMessages') |
| 2 | `component 'Version' does not exist (calling 'getVersion')` | 4 | none |  | `test/tickets/LDEV5601.cfc:44` component 'Version' does not exist (calling 'getVersion') |
| 2 | `component 'java.lang.Double' does not exist (calling 'valueOf')` | 0 | none |  | `test/general/StaticMembersInvoke.cfc:23` component 'java.lang.Double' does not exist (calling 'valueOf') |
| 2 | `component 'lucee.runtime.op.Caster' does not exist (calling 'cfTypeToClass')` | 0 | none |  | `test/tickets/LDEV5708.cfc:10` component 'lucee.runtime.op.Caster' does not exist (calling 'cfTypeToClass') |
| 2 | `component 'lucee.runtime.type.StructImpl' does not exist (calling 'size')` | 0 | none |  | `test/general/NewOperator.cfc:37` component 'lucee.runtime.type.StructImpl' does not exist (calling 'size') |
| 1 | `base component does not resolve; 4 inherited calls not checked` | ? | none |  | `core/src/main/java/resource/component/org/lucee/cfml/test/LuceeTestSuiteRunner.cfc:1` base component does not resolve; 4 inherited calls not checked |
| 1 | `chained on 'a', which is not found (calling 'b')` | 6 | none |  | `test/general/SafeNavigator.cfc:11` chained on 'a', which is not found (calling 'b') |
| 1 | `chained on 'a', which is not found (calling 'c')` | 5 | none |  | `test/general/SafeNavigator.cfc:11` chained on 'a', which is not found (calling 'c') |
| 1 | `chained on 'a', which is not found (calling 'd')` | 2 | none |  | `test/general/SafeNavigator.cfc:11` chained on 'a', which is not found (calling 'd') |
| 1 | `chained on 'authority', which is not found (calling 'build')` | 3 | none |  | `test/tickets/LDEV6084.cfc:17` chained on 'authority', which is not found (calling 'build') |
| 1 | `chained on 'expect', which is not found (calling 'toBeFalse')` | 1 | none |  | `test/tickets/LDEV4694.cfc:21` chained on 'expect', which is not found (calling 'toBeFalse') |
| 1 | `chained on 'expect', which is not found (calling 'toThrow')` | 1 | none |  | `test/tickets/LDEV4694.cfc:18` chained on 'expect', which is not found (calling 'toThrow') |
| 1 | `chained on 'getListener', which is not found (calling 'onIncomingMessage')` | 1 | none |  | `core/src/main/cfml/context/gateway/AsynchronousEvents.cfc:66` chained on 'getListener', which is not found (calling 'onIncomingMessage') |
| 1 | `chained on 'susi', which is not found (calling 'sorglos')` | 0 | none |  | `test/tickets/LDEV4826.cfc:54` chained on 'susi', which is not found (calling 'sorglos') |
| 1 | `component 'ArchiveGreeter' does not exist (calling 'greet')` | 0 | none |  | `test/general/archives/componentPaths/index.cfm:2` component 'ArchiveGreeter' does not exist (calling 'greet') |
| 1 | `component 'AwsBasicCredentials' does not exist (calling 'create')` | 1 | none |  | `test/tickets/LDEV5457.cfc:13` component 'AwsBasicCredentials' does not exist (calling 'create') |
| 1 | `component 'ClientCredentialFactory' does not exist (calling 'createFromSecret')` | 0 | none |  | `test/tickets/LDEV6084.cfc:15` component 'ClientCredentialFactory' does not exist (calling 'createFromSecret') |
| 1 | `component 'ConfidentialClientApplication' does not exist (calling 'builder')` | 0 | none |  | `test/tickets/LDEV6084.cfc:16` component 'ConfidentialClientApplication' does not exist (calling 'builder') |
| 1 | `component 'FileOutputStream' does not exist (calling 'flush')` | 0 | none |  | `test/tickets/LDEV5810.cfc:49` component 'FileOutputStream' does not exist (calling 'flush') |
| 1 | `component 'GreenMail' does not exist (calling 'start')` | 6 | none |  | `test/tags/MailSpool.cfc:39` component 'GreenMail' does not exist (calling 'start') |
| 1 | `component 'GreenMail' does not exist (calling 'stop')` | 5 | none |  | `test/tags/MailSpool.cfc:49` component 'GreenMail' does not exist (calling 'stop') |
| 1 | `component 'HashMap' does not exist (calling 'size')` | 0 | none |  | `test/general/NewOperator.cfc:32` component 'HashMap' does not exist (calling 'size') |
| 1 | `component 'LDEV1706.foo' does not exist (calling 'bar')` | 1 | low | file named foo.cfc (5 candidates) | `test/tickets/_LDEV1706.cfc:27` component 'LDEV1706.foo' does not exist (calling 'bar') |
| 1 | `component 'LDEV1707.foo' does not exist (calling 'bar')` | 1 | low | file named foo.cfc (5 candidates) | `test/tickets/_LDEV1707.cfc:28` component 'LDEV1707.foo' does not exist (calling 'bar') |
| 1 | `component 'LSDateFormat.LSDateFormat' does not exist (calling 'testFunction')` | 9 | medium | file named LSDateFormat.cfc | `test/functions/LSDateFormat.cfc:33` component 'LSDateFormat.LSDateFormat' does not exist (calling 'testFunction') |
| 1 | `component 'LSDateFormat.LSDateFormat' does not exist (calling 'testMemberFunction')` | 16 | medium | file named LSDateFormat.cfc | `test/functions/LSDateFormat.cfc:29` component 'LSDateFormat.LSDateFormat' does not exist (calling 'testMemberFunction') |
| 1 | `component 'MyComponent' does not exist (calling 'someMethod')` | 0 | none |  | `test/tickets/LDEV5836/test-component-methods.cfc:9` component 'MyComponent' does not exist (calling 'someMethod') |
| 1 | `component 'PNGTranscoder' does not exist (calling 'transcode')` | 0 | none |  | `test/tickets/LDEV5810.cfc:48` component 'PNGTranscoder' does not exist (calling 'transcode') |
| 1 | `component 'XWPFDocument' does not exist (calling 'createParagraph')` | 0 | none |  | `test/tickets/LDEV5610.cfc:20` component 'XWPFDocument' does not exist (calling 'createParagraph') |
| 1 | `component 'collection' does not exist (calling 'CREATE')` | 1 | none |  | `test/tickets/_LDEV1684.cfc:12` component 'collection' does not exist (calling 'CREATE') |
| 1 | `component 'dynamiVar' does not exist (calling 'getData')` | 13 | none |  | `test/tickets/LDEV1221/test4.cfm:5` component 'dynamiVar' does not exist (calling 'getData') |
| 1 | `component 'java.lang.System' does not exist (calling 'getenv')` | 2 | none |  | `test/tickets/LDEV5181.cfc:11` component 'java.lang.System' does not exist (calling 'getenv') |
| 1 | `component 'java.util.Collections' does not exist (calling 'emptyList')` | 0 | none |  | `test/general/StaticMembersInvoke.cfc:39` component 'java.util.Collections' does not exist (calling 'emptyList') |
| 1 | `component 'java.util.prefs.BackingStoreException' does not exist (calling 'getMessage')` | 1 | none |  | `test/tickets/LDEV5120.cfc:13` component 'java.util.prefs.BackingStoreException' does not exist (calling 'getMessage') |
| 1 | `component 'lucee.runtime.functions.other.GeneratePBKDFKey' does not exist (calling 'getSupportedAlgorithms')` | 0 | medium | file named GeneratePBKDFKey.cfc | `test/tickets/LDEV0256.cfc:10` component 'lucee.runtime.functions.other.GeneratePBKDFKey' does not exist (calling 'getSupportedAlgorithms') |
| 1 | `component 'lucee.runtime.instrumentation.InstrumentationFactory' does not exist (calling 'getInstrumentation')` | 0 | none |  | `test/run-tests.cfm:4` component 'lucee.runtime.instrumentation.InstrumentationFactory' does not exist (calling 'getInstrumentation') |
| 1 | `component 'objInst' does not exist (calling 'getData')` | 13 | none |  | `test/tickets/LDEV1221/test6.cfm:9` component 'objInst' does not exist (calling 'getData') |
| 1 | `component 'org.apache.poi.Version' does not exist (calling 'getVersion')` | 4 | none |  | `test/tickets/LDEV5601.cfc:73` component 'org.apache.poi.Version' does not exist (calling 'getVersion') |
| 1 | `component 'org.lucee.test.Test' does not exist (calling 'foo')` | 12 | low | file named Test.cfc (56 candidates) | `test/tickets/LDEV4772.cfc:32` component 'org.lucee.test.Test' does not exist (calling 'foo') |
| 1 | `component 'subComponent.TestSubScript$sub' does not exist (calling 'c')` | 5 | none |  | `test/general/SubComponent.cfc:34` component 'subComponent.TestSubScript$sub' does not exist (calling 'c') |
| 1 | `component 'subComponent.TestSubScript$sub' does not exist (calling 'd')` | 2 | none |  | `test/general/SubComponent.cfc:35` component 'subComponent.TestSubScript$sub' does not exist (calling 'd') |
| 1 | `component 'subComponent.TestSubTag$sub' does not exist (calling 'bb')` | 1 | none |  | `test/general/SubComponent.cfc:23` component 'subComponent.TestSubTag$sub' does not exist (calling 'bb') |
| 1 | `component 'subComponent.TestSubTag$sub' does not exist (calling 'subtest')` | 3 | none |  | `test/general/SubComponent.cfc:22` component 'subComponent.TestSubTag$sub' does not exist (calling 'subtest') |
| 1 | `component 'test.testcases.LDEV1102.test' does not exist (calling 'getuserId')` | 4 | low | file named test.cfc (56 candidates) | `test/tickets/_LDEV1102.cfc:7` component 'test.testcases.LDEV1102.test' does not exist (calling 'getuserId') |
| 1 | `component 'test.testcases.LDEV1102.test1' does not exist (calling 'getuserId')` | 4 | low | file named test1.cfc (4 candidates) | `test/tickets/_LDEV1102.cfc:17` component 'test.testcases.LDEV1102.test1' does not exist (calling 'getuserId') |
| 1 | `component 'test.testcases.LDEV1102.test2' does not exist (calling 'getuserId')` | 4 | low | file named test2.cfc (7 candidates) | `test/tickets/_LDEV1102.cfc:27` component 'test.testcases.LDEV1102.test2' does not exist (calling 'getuserId') |
| 1 | `component 'testComp$testSub' does not exist (calling 'addiFunc')` | 0 | none |  | `test/general/subComponent/index.cfm:14` component 'testComp$testSub' does not exist (calling 'addiFunc') |
| 1 | `component 'testComp$testSub' does not exist (calling 'subFunc')` | 0 | none |  | `test/general/subComponent/index.cfm:8` component 'testComp$testSub' does not exist (calling 'subFunc') |
| 1 | `component 'testbox.system.reports.JUnitReporter' does not exist (calling 'runReport')` | 0 | none |  | `test/run-tests.cfm:444` component 'testbox.system.reports.JUnitReporter' does not exist (calling 'runReport') |

<details><summary>Groups with several candidates</summary>

- `component 'LDEV0273.Test' does not exist (calling 'exclaim')` — 3 finding(s), 56 candidate(s):
  - low `test/tickets/LDEV0078/Test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0104/Test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0255/Test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0280/test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0421/test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0460/Test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0486/test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0581/Test.cfc` — file named Test.cfc
  - … 48 more
- `component 'LDEV0273.Test' does not exist (calling 'exclaim2')` — 3 finding(s), 56 candidate(s):
  - low `test/tickets/LDEV0078/Test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0104/Test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0255/Test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0280/test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0421/test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0460/Test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0486/test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0581/Test.cfc` — file named Test.cfc
  - … 48 more
- `component 'LDEV1706.foo' does not exist (calling 'bar')` — 1 finding(s), 5 candidate(s):
  - low `test/tickets/LDEV1659/model/Foo.cfc` — file named foo.cfc
  - low `test/tickets/LDEV3900/otherCfc/foo.cfc` — file named foo.cfc
  - low `test/tickets/LDEV1741/App1/model/Foo.cfc` — file named foo.cfc
  - low `test/tickets/LDEV1741/App2/model/Foo.cfc` — file named foo.cfc
  - low `test/tickets/LDEV1984/App1/model/Foo.cfc` — file named foo.cfc
- `component 'LDEV1707.foo' does not exist (calling 'bar')` — 1 finding(s), 5 candidate(s):
  - low `test/tickets/LDEV1659/model/Foo.cfc` — file named foo.cfc
  - low `test/tickets/LDEV3900/otherCfc/foo.cfc` — file named foo.cfc
  - low `test/tickets/LDEV1741/App1/model/Foo.cfc` — file named foo.cfc
  - low `test/tickets/LDEV1741/App2/model/Foo.cfc` — file named foo.cfc
  - low `test/tickets/LDEV1984/App1/model/Foo.cfc` — file named foo.cfc
- `component 'org.lucee.test.Test' does not exist (calling 'foo')` — 1 finding(s), 56 candidate(s):
  - low `test/tickets/LDEV0078/Test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0104/Test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0255/Test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0280/test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0421/test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0460/Test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0486/test.cfc` — file named Test.cfc
  - low `test/tickets/LDEV0581/Test.cfc` — file named Test.cfc
  - … 48 more
- `component 'test.testcases.LDEV1102.test' does not exist (calling 'getuserId')` — 1 finding(s), 56 candidate(s):
  - low `test/tickets/LDEV0078/Test.cfc` — file named test.cfc
  - low `test/tickets/LDEV0104/Test.cfc` — file named test.cfc
  - low `test/tickets/LDEV0255/Test.cfc` — file named test.cfc
  - low `test/tickets/LDEV0280/test.cfc` — file named test.cfc
  - low `test/tickets/LDEV0421/test.cfc` — file named test.cfc
  - low `test/tickets/LDEV0460/Test.cfc` — file named test.cfc
  - low `test/tickets/LDEV0486/test.cfc` — file named test.cfc
  - low `test/tickets/LDEV0581/Test.cfc` — file named test.cfc
  - … 48 more
- `component 'test.testcases.LDEV1102.test1' does not exist (calling 'getuserId')` — 1 finding(s), 4 candidate(s):
  - low `test/tickets/LDEV1102/test1.cfc` — file named test1.cfc
  - low `test/tickets/LDEV1152/test1.cfc` — file named test1.cfc
  - low `test/tickets/LDEV1589/test1.cfc` — file named test1.cfc
  - low `test/tickets/LDEV2645/test1.cfc` — file named test1.cfc
- `component 'test.testcases.LDEV1102.test2' does not exist (calling 'getuserId')` — 1 finding(s), 7 candidate(s):
  - low `test/tickets/LDEV1102/test2.cfc` — file named test2.cfc
  - low `test/tickets/LDEV1152/test2.cfc` — file named test2.cfc
  - low `test/tickets/LDEV1202/Test2.cfc` — file named test2.cfc
  - low `test/tickets/LDEV1431/test2.cfc` — file named test2.cfc
  - low `test/tickets/LDEV1589/test2.cfc` — file named test2.cfc
  - low `test/tickets/LDEV2862/test2.cfc` — file named test2.cfc
  - low `test/tickets/LDEV3634/test2.cfc` — file named test2.cfc

</details>
