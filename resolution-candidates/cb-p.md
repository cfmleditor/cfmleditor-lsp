# cb-p — unresolved findings and candidates

Every finding below is still reported. A candidate is a liberal match offered for a person to weigh, never an answer the resolver took: **high** is the only component declaring every method the function calls on the receiver (or the only one, named like it); **medium** is a sole match on weaker evidence, or the one named like the receiver among several; **low** is one of several. *Defined* is how many indexed files declare a method of that name: 0 means it is missing from the workspace, more means the resolver could not connect the call to it.

| Category | Findings | high | medium | low | none | method defined nowhere |
|---|---:|---:|---:|---:|---:|---:|
| variable | 1223 | 560 | 171 | 127 | 365 | 73 |
| return-type | 69 | 9 | 34 | 18 | 8 | 4 |
| method | 155 | 0 | 0 | 75 | 80 | 40 |
| object | 97 | 0 | 0 | 0 | 97 | 82 |

## Variable definitions — a receiver whose component is unknown

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 41 | `thiscontent → modules/contentbox/models/content/BaseContent.cfc` | 1 | medium | declares removeCategories() | `modules/contentbox/models/content/CategoryService.cfc:324` variable 'thisContent' has no component ref |
| 35 | `arguments.content → modules/contentbox/models/content/BaseContent.cfc` | 3 | high | declares hasParent(), getParent(), hasCategories(), removeAllCategories(), hasRelatedContent(), getRelatedContent(), hasLinkedContent(), removeAllLinkedConte… | `modules/contentbox/models/content/ContentService.cfc:417` variable 'arguments.content' has no component ref |
| 26 | `exporter` | 8 | none |  | `modules/contentbox/models/exporters/ContentBoxExporter.cfc:296` variable 'exporter' has no component ref |
| 25 | `entry` | 7 | none |  | `modules/contentbox/models/system/NotificationService.cfc:164` variable 'entry' has no component ref |
| 24 | `content → modules/contentbox/models/content/BaseContent.cfc` | 1 | medium | declares getCommentSubscriptions() | `modules/contentbox/models/comments/CommentService.cfc:247` variable 'content' has no component ref |
| 24 | `rc.comment → modules/contentbox/models/comments/Comment.cfc` | 3 | high | declares getAuthorEmail(), getAuthor(), getRelatedContent(), getAuthorURL(), getAuthorIP(), getDisplayCreatedDate(), getDisplayContent(); named like the rece… | `modules/contentbox/modules/contentbox-admin/views/comments/moderate.cfm:30` variable 'rc.comment' has no component ref |
| 23 | `args.menuitem` | 1 | none |  | `modules/contentbox/models/menu/views/content/display.cfm:2` variable 'args.menuItem' has no component ref |
| 22 | `arguments.setup → modules/contentbox-installer/models/Setup.cfc` | 1 | high | declares getFullRewrite(), getPopulateData(), getCreateDevSite(); named like the receiver 'setup' | `modules/contentbox-installer/models/InstallerService.cfc:41` variable 'arguments.setup' has no component ref |
| 22 | `c` | 1 | none |  | `modules/contentbox/models/content/ContentService.cfc:510` variable 'c' has no component ref |
| 21 | `incomment → modules/contentbox/models/comments/Comment.cfc` | 7 | high | declares getRelatedContent(), setAuthorIP(), setIsApproved(), getIsApproved(), getMemento() | `modules/contentbox/models/comments/CommentService.cfc:197` variable 'inComment' has no component ref |
| 21 | `prc.cbhelper → modules/contentbox/models/system/CBHelper.cfc` | 1 | high | declares adminRoot(), siteRoot(); named like the receiver 'cbHelper' | `modules/contentbox/modules/contentbox-admin/layouts/admin.cfm:21` variable 'prc.cbHelper' has no component ref |
| 21 | `response` | 0 | none |  | `modules/contentbox/models/media/ForwardMediaProvider.cfc:34` variable 'response' has no component ref |
| 20 | `vresults` | 0 | none |  | `modules/contentbox/modules/contentbox-admin/handlers/authors.cfc:347` variable 'vResults' has no component ref |
| 19 | `arguments.original → modules/contentbox/models/content/BaseContent.cfc` | 1 | high | declares getHTMLKeywords(), getHTMLDescription(), getHTMLTitle(), getMarkup(), getCache(), getCacheTimeout(), getCacheLastAccessTimeout(), getShowInSearch(),… | `modules/contentbox/models/content/BaseContent.cfc:1324` variable 'arguments.original' has no component ref |
| 19 | `arguments.site → modules/contentbox/models/system/Site.cfc` | 5 | medium | declares getsiteID(); named like the receiver 'site' (5 candidates) | `modules/contentbox/models/content/BaseContent.cfc:1784` variable 'arguments.site' has no component ref |
| 19 | `prc.author → modules/contentbox/models/security/Author.cfc` | 2 | high | declares getUsername(), getEmail(), isLoaded(), getAuthorID(); named like the receiver 'author' | `modules/contentbox/modules/contentbox-admin/views/authors/editorHelper.cfm:38` variable 'prc.author' has no component ref |
| 18 | `content` | 7 | none |  | `modules/contentbox/models/system/NotificationService.cfc:426` variable 'content' has no component ref |
| 18 | `setting → modules/contentbox/models/system/Setting.cfc` | 1 | high | declares getSettingID(), getName(), getValue(), getIsCore(), hasSite(), getSite(), getsiteID(); named like the receiver 'setting' | `modules/contentbox/modules/contentbox-admin/views/settings/rawSettingsTable.cfm:25` variable 'setting' has no component ref |
| 18 | `thisperm → modules/contentbox/models/security/Permission.cfc` | 1 | high | declares getPermission(), getPermissionID() | `modules/contentbox/modules/contentbox-admin/views/authors/permissions.cfm:151` variable 'thisPerm' has no component ref |
| 17 | `permission → modules/contentbox/models/security/Permission.cfc` | 1 | high | declares getpermissionID(), getPermission(), getDescription(), getNumberOfRoles(), getNumberOfGroups(); named like the receiver 'permission' | `modules/contentbox/modules/contentbox-admin/views/permissions/index.cfm:111` variable 'permission' has no component ref |
| 14 | `args.content` | 1 | none |  | `modules/contentbox/modules/contentbox-admin/views/_components/content/TableCreationInfo.cfm:6` variable 'args.content' has no component ref |
| 14 | `arguments.page → modules/contentbox/models/content/BaseContent.cfc` | 5 | low | declares getSlug() (5 candidates) | `modules/contentbox/models/content/PageService.cfc:45` variable 'arguments.page' has no component ref |
| 14 | `author → modules/contentbox/models/security/Author.cfc` | 1 | high | declares getFullName(), getRole(), getPermissionGroupsList(), getEmail(), getAuthorID(); named like the receiver 'author' | `modules/contentbox/models/system/NotificationService.cfc:56` variable 'author' has no component ref |
| 13 | `arguments.author → modules/contentbox/models/security/Author.cfc` | 3 | high | declares clearPermissions(), clearPermissionGroups(); named like the receiver 'author' | `modules/contentbox/models/security/AuthorService.cfc:169` variable 'arguments.author' has no component ref |
| 13 | `arguments.data.content → modules/contentbox/models/content/BaseContent.cfc` | 1 | medium | declares getMarkup() | `modules/contentbox/models/content/renderers/MarkdownRenderer.cfc:26` variable 'arguments.data.content' has no component ref |
| 13 | `comment → modules/contentbox/models/comments/Comment.cfc` | 3 | medium | declares getAuthorEmail(), getRelatedContent(); named like the receiver 'comment' (3 candidates) | `modules/contentbox/models/subscriptions/SubscriptionListener.cfc:35` variable 'comment' has no component ref |
| 13 | `expectation.actual` | 2 | none |  | `tests/resources/BaseApiTest.cfc:51` variable 'expectation.actual' has no component ref |
| 13 | `prc.activedisk` | 0 | none |  | `modules/contentbox/modules/contentbox-admin/modules/contentbox-filebrowser/handlers/Editor.cfc:29` variable 'prc.activeDisk' has no component ref |
| 13 | `site → modules/contentbox/models/system/Site.cfc` | 1 | high | declares getIsSSL(); named like the receiver 'site' | `modules/contentbox/models/comments/CommentService.cfc:506` variable 'site' has no component ref |
| 12 | `item` | 3 | none |  | `modules/contentbox/models/menu/MenuService.cfc:281` variable 'item' has no component ref |
| 12 | `ocurrentcontent → modules/contentbox/models/content/BaseContent.cfc` | 1 | high | declares getHTMLTitle(), getTitle() | `modules/contentbox/models/system/CBHelper.cfc:909` variable 'oCurrentContent' has no component ref |
| 12 | `prc.entry → modules/contentbox/models/content/BaseContent.cfc` | 1 | medium | declares getComments() | `modules/contentbox/models/exporters/StaticExporter.cfc:188` variable 'prc.entry' has no component ref |
| 11 | `args.ocontent` | 1 | none |  | `modules/contentbox/modules/contentbox-ui/views/adminbar/index.cfm:33` variable 'args.oContent' has no component ref |
| 11 | `arguments.c` | 1 | none |  | `modules/contentbox/models/content/ContentService.cfc:518` variable 'arguments.c' has no component ref |
| 11 | `group → modules/contentbox/models/security/PermissionGroup.cfc` | 1 | high | declares getPermissionGroupID(), getName(), getPermissions() | `modules/contentbox/modules/contentbox-admin/views/authors/permissions.cfm:99` variable 'group' has no component ref |
| 11 | `osite → modules/contentbox/models/system/Site.cfc` | 2 | high | declares isLoaded(), getSlug(), removeAllSettings(), setSettings(), setCategories(); named like the receiver 'oSite' | `modules/contentbox/models/system/SiteService.cfc:471` variable 'oSite' has no component ref |
| 11 | `role → modules/contentbox/models/security/Role.cfc` | 1 | high | declares getRoleID(), getName(), getRole(), getDescription(), getNumberOfPermissions(), getNumberOfAuthors(); named like the receiver 'role' | `modules/contentbox/modules/contentbox-admin/views/roles/index.cfm:121` variable 'role' has no component ref |
| 10 | `category → modules/contentbox/models/content/Category.cfc` | 5 | high | declares getCategory(), getCategoryID(); named like the receiver 'category' | `modules/contentbox/modules/contentbox-admin/views/contentStore/index.cfm:188` variable 'category' has no component ref |
| 9 | `ocontent → modules/contentbox/models/content/BaseContent.cfc` | 10 | high | declares getMemento(), getCache(), isContentPublished(), isLoaded(), getContentID(), getCacheTimeout(), getCacheLastAccessTimeout() | `modules/contentbox/modules/contentbox-ui/handlers/content.cfc:290` variable 'oContent' has no component ref |
| 9 | `page` | 7 | none |  | `modules/contentbox/models/system/NotificationService.cfc:295` variable 'page' has no component ref |
| 9 | `prc.page` | 1 | none |  | `modules/contentbox/modules/contentbox-ui/handlers/page.cfc:65` variable 'prc.page' has no component ref |
| 8 | `arguments.category → modules/contentbox/models/content/Category.cfc` | 5 | medium | declares getSlug(), getCategory(); named like the receiver 'category' (2 candidates) | `modules/contentbox/models/content/CategoryService.cfc:96` variable 'arguments.category' has no component ref |
| 8 | `arguments.comment → modules/contentbox/models/comments/Comment.cfc` | 7 | medium | declares getRelatedContent(), getAuthorEmail(), getMemento(), getParentTitle(); named like the receiver 'comment' (2 candidates) | `modules/contentbox/models/comments/CommentService.cfc:245` variable 'arguments.comment' has no component ref |
| 8 | `arguments.content → modules/contentbox/models/content/Page.cfc` | 1 | high | declares getLayoutWithInheritance(), renderContent(), getSite(), getSlug(), hasChild(), getChildren() | `modules/contentbox/models/exporters/StaticExporter.cfc:318` variable 'arguments.content' has no component ref |
| 8 | `requestservice` | 2 | none |  | `modules/contentbox/models/ui/AdminMenuService.cfc:203` variable 'requestService' has no component ref |
| 8 | `thisitem → modules/contentbox/models/search/SearchResults.cfc` | 10 | medium | declares getMemento() | `modules/contentbox/models/content/CategoryService.cfc:338` variable 'thisItem' has no component ref |
| 7 | `args.content → modules/contentbox/models/content/BaseContent.cfc` | 1 | medium | declares getShowInSearch() | `modules/contentbox/modules/contentbox-admin/views/_components/content/TableSearchStatus.cfm:2` variable 'args.content' has no component ref |
| 7 | `arguments.menuitem → modules/contentbox/models/menu/item/ContentMenuItem.cfc` | 1 | high | declares getContentSlug(), getMenu() | `modules/contentbox/models/menu/providers/ContentProvider.cfc:54` variable 'arguments.menuItem' has no component ref |
| 7 | `arguments.module → modules/contentbox/models/modules/Module.cfc` | 4 | high | declares setTitle(), setAuthor(), setWebURL(), setDescription(), setVersion(), setEntryPoint(), setForgeBoxSlug(); named like the receiver 'module' | `modules/contentbox/models/modules/ModuleService.cfc:117` variable 'arguments.module' has no component ref |
| 7 | `c.restrictions` | 1 | none |  | `modules/contentbox/models/content/ContentService.cfc:513` variable 'c.restrictions' has no component ref |
| 7 | `entryresults.content[]` | 5 | none |  | `modules/contentbox/models/rss/RSSService.cfc:212` variable 'entryResults.content[]' has no component ref |
| 7 | `pageresults.content[]` | 5 | none |  | `modules/contentbox/models/rss/RSSService.cfc:287` variable 'pageResults.content[]' has no component ref |
| 7 | `prc.response` | 1 | none |  | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/auth.cfc:30` variable 'prc.response' has no component ref |
| 7 | `settingservice → modules/contentbox/models/system/SettingService.cfc` | 2 | medium | declares getAllSettings(); named like the receiver 'settingService' (2 candidates) | `modules/contentbox/models/rss/RSSService.cfc:36` variable 'settingService' has no component ref |
| 7 | `thisauthor → modules/contentbox/models/security/Author.cfc` | 2 | high | declares getEmail(), getRole(), getAUthorID(), getFullName() | `modules/contentbox/modules/contentbox-admin/views/content/search.cfm:66` variable 'thisAuthor' has no component ref |
| 6 | `arguments.target → modules/contentbox/models/content/BaseContent.cfc` | 50 | high | declares getContentService(), getContentType(), getContentID(), isLoaded(), hasSite(), getSite() | `modules/contentbox/models/content/BaseContent.cfc:568` variable 'arguments.target' has no component ref |
| 6 | `arguments.thiscontent → modules/contentbox/models/content/BaseContent.cfc` | 3 | low | declares hasParent() (2 candidates) | `modules/contentbox/models/system/Site.cfc:573` variable 'arguments.thisContent' has no component ref |
| 6 | `arguments.thisitem → modules/contentbox/models/content/BaseContent.cfc` | 5 | low | declares getInfoSnapshot() (5 candidates) | `modules/contentbox/models/content/BaseContent.cfc:873` variable 'arguments.thisItem' has no component ref |
| 6 | `contentresults.content[] → modules/contentbox/models/content/BaseContent.cfc` | 3 | high | declares getAuthorEmail(), getAuthorName(), getCategoriesList(), renderContent() | `modules/contentbox/models/rss/RSSService.cfc:362` variable 'contentResults.content[]' has no component ref |
| 6 | `entries` | 0 | none |  | `modules/contentbox/models/util/ZipUtil.cfc:203` variable 'entries' has no component ref |
| 6 | `instance.zipoutput` | 0 | none |  | `modules/contentbox/models/util/ZipUtil.cfc:70` variable 'instance.zipOutput' has no component ref |
| 6 | `menuitem → modules/contentbox/models/menu/item/ContentMenuItem.cfc` | 1 | medium | declares setContentSlug() | `modules/contentbox/modules/contentbox-admin/interceptors/MenuCleanup.cfc:27` variable 'menuItem' has no component ref |
| 6 | `owidget → modules/contentbox/models/ui/Widget.cfc` | 3 | medium | declares getAuthorURL(), getName(), getDescription(), getVersion(), getAuthor(); named like the receiver 'oWidget' (2 candidates) | `modules/contentbox/modules/contentbox-admin/views/widgets/widgetList.cfm:112` variable 'oWidget' has no component ref |
| 5 | `args.menuitem → modules/contentbox/models/menu/item/BaseMenuItem.cfc` | 1 | medium | declares getLabel() | `modules/contentbox/models/menu/views/free/display.cfm:2` variable 'args.menuItem' has no component ref |
| 5 | `arguments.target → modules/contentbox/models/content/Category.cfc` | 25 | high | declares getCategoryService(), getCategoryID(), isLoaded(), hasSite(), getSite() | `modules/contentbox/models/content/Category.cfc:120` variable 'arguments.target' has no component ref |
| 5 | `entry → modules/contentbox/models/content/Entry.cfc` | 7 | medium | declares getSite(), getTitle(), hasExcerpt(), renderExcerpt(), renderContent(); named like the receiver 'entry' (2 candidates) | `modules/contentbox/models/system/NotificationService.cfc:229` variable 'entry' has no component ref |
| 5 | `instance.zipfile` | 0 | none |  | `modules/contentbox/models/util/ZipUtil.cfc:196` variable 'instance.zipFile' has no component ref |
| 5 | `item → modules/contentbox/models/content/BaseContent.cfc` | 4 | high | declares getTitle(), renderContent(), getContentType(), hasCategories(), getCategoriesList() | `modules/contentbox/models/search/DBSearch.cfc:139` variable 'item' has no component ref |
| 5 | `locpage → modules/contentbox/models/content/BaseContent.cfc` | 1 | high | declares getContentID(), hasParent(), getParent() | `modules/contentbox/models/system/CBHelper.cfc:2459` variable 'locPage' has no component ref |
| 5 | `ouser → modules/contentbox/models/security/Author.cfc` | 4 | high | declares setPermissions(), addPermissionGroup(), setRole(), isLoaded() | `modules/contentbox/models/security/AuthorService.cfc:481` variable 'oUser' has no component ref |
| 5 | `page → modules/contentbox/models/content/Page.cfc` | 7 | medium | declares getSite(), getTitle(), hasExcerpt(), renderExcerpt(), renderContent(); named like the receiver 'page' (2 candidates) | `modules/contentbox/models/system/NotificationService.cfc:359` variable 'page' has no component ref |
| 5 | `prc.response → modules/contentbox/models/menu/item/BaseMenuItem.cfc` | 2 | medium | declares setData() | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/auth.cfc:117` variable 'prc.response' has no component ref |
| 4 | `args.provider` | 12 | none |  | `modules/contentbox/modules/contentbox-admin/views/menus/provider.cfm:11` variable 'args.provider' has no component ref |
| 4 | `arguments.categories[] → modules/contentbox/models/content/Category.cfc` | 5 | low | declares getCategory(), getNumberOfEntries() (2 candidates) | `modules/contentbox/widgets/Categories.cfc:86` variable 'arguments.categories[]' has no component ref |
| 4 | `arguments.original → modules/contentbox/models/content/Page.cfc` | 1 | high | declares getLayout(), getShowInMenu(), hasExcerpt(), getExcerpt() | `modules/contentbox/models/content/Page.cfc:183` variable 'arguments.original' has no component ref |
| 4 | `arguments.populate.model → modules/contentbox/models/content/BaseContent.cfc` | 2 | high | declares setCreator(), isLoaded(), getSlug() | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/baseContentHandler.cfc:129` variable 'arguments.populate.model' has no component ref |
| 4 | `arguments.target` | 5 | none |  | `modules/contentbox/models/validators/UniqueSiteFieldValidator.cfc:48` variable 'arguments.target' has no component ref |
| 4 | `arguments.task` | 4 | none |  | `config/Scheduler.cfc:60` variable 'arguments.task' has no component ref |
| 4 | `arguments.thissite → modules/contentbox/models/content/BaseContent.cfc` | 5 | low | declares getSlug() (5 candidates) | `modules/contentbox/config/Scheduler.cfc:41` variable 'arguments.thisSite' has no component ref |
| 4 | `item → modules/contentbox/models/system/Setting.cfc` | 6 | high | declares hasSite(), getName(), getSite(), getValue() | `modules/contentbox/models/system/SettingService.cfc:582` variable 'item' has no component ref |
| 4 | `original → modules/contentbox/models/content/BaseContent.cfc` | 5 | high | declares getTitle(), isSameSite(), getParent(), getSlug(), hasParent() | `modules/contentbox/modules/contentbox-admin/handlers/baseContentHandler.cfc:588` variable 'original' has no component ref |
| 4 | `perm → modules/contentbox/models/security/Permission.cfc` | 1 | high | declares getPermission(), getPermissionID() | `modules/contentbox/modules/contentbox-admin/views/authors/permissions.cfm:21` variable 'perm' has no component ref |
| 4 | `prc.oentity → modules/contentbox/models/content/BaseContent.cfc` | 1 | high | declares addNewContentVersion(), setCategories(), setCustomFields(), getMemento() | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/baseContentHandler.cfc:162` variable 'prc.oEntity' has no component ref |
| 4 | `prc.opaging → modules/contentbox/models/ui/Paging.cfc` | 26 | medium | declares renderit(); named like the receiver 'oPaging' (25 candidates) | `modules/contentbox/models/system/CBHelper.cfc:1219` variable 'prc.oPaging' has no component ref |
| 4 | `prc.page → modules/contentbox/models/content/BaseContent.cfc` | 4 | high | declares getTitle(), getSlug(), renderContent(), getNumberOfApprovedComments() | `modules/contentbox/themes/default/views/page.cfm:50` variable 'prc.page' has no component ref |
| 4 | `stat → modules/contentbox/models/content/Stats.cfc` | 7 | high | declares getRelatedContent(), getHits() | `modules/contentbox/models/content/ContentService.cfc:791` variable 'stat' has no component ref |
| 3 | `args.ocurrentauthor → modules/contentbox/models/security/Author.cfc` | 2 | high | declares getEmail(), getFullName() | `modules/contentbox/modules/contentbox-ui/views/adminbar/index.cfm:144` variable 'args.oCurrentAuthor' has no component ref |
| 3 | `arguments.author` | 2 | none |  | `modules/contentbox/models/security/AuthorService.cfc:225` variable 'arguments.author' has no component ref |
| 3 | `arguments.c.restrictions` | 1 | none |  | `modules/contentbox/models/content/ContentService.cfc:535` variable 'arguments.c.restrictions' has no component ref |
| 3 | `arguments.criteria` | 1 | none |  | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/baseHandler.cfc:66` variable 'arguments.criteria' has no component ref |
| 3 | `arguments.entity` | 2 | none |  | `modules/contentbox/models/security/SecurityRuleService.cfc:55` variable 'arguments.entity' has no component ref |
| 3 | `arguments.entry → modules/contentbox/models/system/Site.cfc` | 5 | low | declares getSlug() (5 candidates) | `modules/contentbox/models/system/CBHelper.cfc:1433` variable 'arguments.entry' has no component ref |
| 3 | `arguments.menuitem → modules/contentbox/models/search/SearchResults.cfc` | 10 | medium | declares getMemento() | `modules/contentbox/models/menu/providers/FreeProvider.cfc:55` variable 'arguments.menuItem' has no component ref |
| 3 | `arguments.page → modules/contentbox/models/system/Site.cfc` | 5 | low | declares getSlug() (5 candidates) | `modules/contentbox/models/system/CBHelper.cfc:1510` variable 'arguments.page' has no component ref |
| 3 | `arguments.parent → modules/contentbox/models/content/BaseContent.cfc` | 2 | high | declares addChild(), getSlug() | `modules/contentbox/models/content/BaseContent.cfc:1803` variable 'arguments.parent' has no component ref |
| 3 | `arguments.site` | 2 | none |  | `modules/contentbox/models/system/SiteService.cfc:615` variable 'arguments.site' has no component ref |
| 3 | `arguments.thischild → modules/contentbox/models/content/BaseContent.cfc` | 4 | low | declares getTitle(), getSlug() (2 candidates) | `modules/contentbox/models/content/BaseContent.cfc:1415` variable 'arguments.thisChild' has no component ref |
| 3 | `arguments.thiscomment → modules/contentbox/models/content/BaseContent.cfc` | 1 | medium | declares buildContentCacheCleanupKey() | `modules/contentbox/models/content/util/ContentCacheCleanup.cfc:38` variable 'arguments.thisComment' has no component ref |
| 3 | `arguments.thisitem → modules/contentbox/models/search/SearchResults.cfc` | 10 | medium | declares getMemento() | `modules/contentbox/models/content/ContentService.cfc:833` variable 'arguments.thisItem' has no component ref |
| 3 | `commentresults.comments[] → modules/contentbox/models/comments/Comment.cfc` | 2 | low | declares getParentTitle() (2 candidates) | `modules/contentbox/models/rss/RSSService.cfc:433` variable 'commentResults.comments[]' has no component ref |
| 3 | `item → modules/contentbox/models/security/Permission.cfc` | 1 | medium | declares getPermission() | `modules/contentbox/models/security/Author.cfc:333` variable 'item' has no component ref |
| 3 | `local.thissubmenu` | 1 | none |  | `modules/contentbox/models/ui/templates/navAdminMenu.cfm:84` variable 'local.thisSubMenu' has no component ref |
| 3 | `local.topmenu` | 1 | none |  | `modules/contentbox/models/ui/templates/navAdminMenu.cfm:20` variable 'local.topMenu' has no component ref |
| 3 | `menu → modules/contentbox/models/menu/Menu.cfc` | 5 | medium | declares getSlug(), getTitle(); named like the receiver 'menu' (2 candidates) | `modules/contentbox/modules/contentbox-admin/views/menus/providers/submenu/admin.cfm:9` variable 'menu' has no component ref |
| 3 | `ocategory → modules/contentbox/models/content/BaseContent.cfc` | 6 | high | declares setSite(), isLoaded() | `modules/contentbox/models/content/CategoryService.cfc:462` variable 'oCategory' has no component ref |
| 3 | `ogroup` | 4 | none |  | `modules/contentbox/models/security/PermissionGroupService.cfc:125` variable 'oGroup' has no component ref |
| 3 | `orole` | 4 | none |  | `modules/contentbox/models/security/RoleService.cfc:127` variable 'oRole' has no component ref |
| 3 | `otemplate → modules/contentbox/models/content/BaseContent.cfc` | 6 | high | declares setSite(), isLoaded() | `modules/contentbox/models/content/ContentTemplateService.cfc:311` variable 'oTemplate' has no component ref |
| 3 | `prc.activedisk → modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/authors.cfc` | 12 | low | declares create() (11 candidates) | `modules/contentbox/modules/contentbox-admin/modules/contentbox-filebrowser/handlers/Editor.cfc:214` variable 'prc.activeDisk' has no component ref |
| 3 | `prc.adminmenuservice → modules/contentbox/models/ui/AdminMenuService.cfc` | 1 | high | declares generateUtilsMenu(), generateProfileMenu(), generateMenu(); named like the receiver 'adminMenuService' | `modules/contentbox/modules/contentbox-admin/layouts/admin.cfm:244` variable 'prc.adminMenuService' has no component ref |
| 3 | `prc.oeditordriver → modules/contentbox/modules/contentbox-admin/modules/contentbox-ckeditor/models/CKEditor.cfc` | 9 | low | declares startup(), shutdown(), loadAssets() (4 candidates) | `modules/contentbox/modules/contentbox-admin/views/content/editorHelper.cfm:30` variable 'prc.oEditorDriver' has no component ref |
| 3 | `prc.owidget → modules/contentbox/models/modules/Module.cfc` | 8 | low | declares getName(), getVersion(), getForgeBoxSlug(), getDescription() (2 candidates) | `modules/contentbox/modules/contentbox-admin/views/widgets/docs.cfm:15` variable 'prc.oWidget' has no component ref |
| 3 | `thispage → modules/contentbox/models/content/BaseContent.cfc` | 5 | low | declares getSlug(), setSlug() (5 candidates) | `modules/contentbox/models/content/PageService.cfc:40` variable 'thisPage' has no component ref |
| 2 | `activecontent → modules/contentbox/models/content/ContentVersion.cfc` | 6 | high | declares getAuthor(), hasAuthor() | `modules/contentbox/models/content/BaseContent.cfc:661` variable 'activeContent' has no component ref |
| 2 | `args.menuitem → modules/contentbox/models/menu/item/MediaMenuItem.cfc` | 1 | medium | declares getMediaPath() | `modules/contentbox/modules/contentbox-admin/views/menus/providers/media/admin.cfm:10` variable 'args.menuItem' has no component ref |
| 2 | `arguments.content → modules/contentbox/models/system/Site.cfc` | 1 | high | declares getSiteRoot(); named like the receiver 'Site' | `modules/contentbox/models/exporters/StaticExporter.cfc:273` variable 'arguments.content' has no component ref |
| 2 | `arguments.data.menu → modules/contentbox/models/menu/Menu.cfc` | 5 | medium | declares getSlug(); named like the receiver 'menu' (5 candidates) | `modules/contentbox/modules/contentbox-admin/interceptors/MenuCleanup.cfc:100` variable 'arguments.data.menu' has no component ref |
| 2 | `arguments.menu` | 1 | none |  | `modules/contentbox/models/ui/AdminMenuService.cfc:156` variable 'arguments.menu' has no component ref |
| 2 | `arguments.menuitem → modules/contentbox/models/menu/Menu.cfc` | 5 | medium | declares getSiteID(); named like the receiver 'Menu' (5 candidates) | `modules/contentbox/models/menu/providers/ContentProvider.cfc:56` variable 'arguments.menuItem' has no component ref |
| 2 | `arguments.menuitem → modules/contentbox/models/menu/item/SubMenuItem.cfc` | 1 | medium | declares getMenuSlug() | `modules/contentbox/models/menu/providers/SubMenuProvider.cfc:53` variable 'arguments.menuItem' has no component ref |
| 2 | `arguments.original → modules/contentbox/models/content/ContentStore.cfc` | 12 | high | declares getDescription(), getOrder() | `modules/contentbox/models/content/ContentStore.cfc:98` variable 'arguments.original' has no component ref |
| 2 | `arguments.original → modules/contentbox/models/content/Entry.cfc` | 2 | low | declares hasExcerpt(), getExcerpt() (2 candidates) | `modules/contentbox/models/content/Entry.cfc:126` variable 'arguments.original' has no component ref |
| 2 | `arguments.originalservice → modules/contentbox/models/content/CategoryService.cfc` | 24 | low | declares save() (23 candidates) | `modules/contentbox/models/content/BaseContent.cfc:1405` variable 'arguments.originalService' has no component ref |
| 2 | `arguments.relatedcontent[] → modules/contentbox/models/content/BaseContent.cfc` | 4 | low | declares getTitle() (4 candidates) | `modules/contentbox/widgets/RelatedContent.cfc:80` variable 'arguments.relatedContent[]' has no component ref |
| 2 | `arguments.site → modules/contentbox/models/content/BaseContent.cfc` | 2 | high | declares isLoaded(), getsiteID() | `modules/contentbox/models/content/CategoryService.cfc:129` variable 'arguments.site' has no component ref |
| 2 | `arguments.target → modules/contentbox/models/system/Site.cfc` | 5 | medium | declares getSiteID(); named like the receiver 'Site' (5 candidates) | `modules/contentbox/models/content/BaseContent.cfc:576` variable 'arguments.target' has no component ref |
| 2 | `arguments.thisentry → modules/contentbox/models/search/SearchResults.cfc` | 10 | medium | declares getMemento() | `modules/contentbox/modules/contentbox-ui/handlers/blog.cfc:147` variable 'arguments.thisEntry' has no component ref |
| 2 | `arguments.thisfield → modules/contentbox/models/content/CustomField.cfc` | 3 | high | declares getKey(), getValue() | `modules/contentbox/models/content/BaseContent.cfc:1374` variable 'arguments.thisField' has no component ref |
| 2 | `cat → modules/contentbox/models/content/Category.cfc` | 1 | medium | declares getCategoryID() | `modules/contentbox/models/content/BaseContent.cfc:1853` variable 'cat' has no component ref |
| 2 | `cats[] → modules/contentbox/models/system/Site.cfc` | 5 | low | declares getCategory() (4 candidates) | `modules/contentbox/models/system/CBHelper.cfc:1848` variable 'cats[]' has no component ref |
| 2 | `cbmessagebox` | 3 | none |  | `modules/contentbox/modules/contentbox-admin/handlers/contentTemplates.cfc:118` variable 'cbMessagebox' has no component ref |
| 2 | `clone → modules/contentbox/models/content/BaseContent.cfc` | 9 | high | declares clone(), getSlug() | `modules/contentbox/modules/contentbox-admin/handlers/baseContentHandler.cfc:583` variable 'clone' has no component ref |
| 2 | `content → modules/contentbox/models/comments/Comment.cfc` | 4 | low | declares getContent() (4 candidates) | `modules/contentbox/models/system/NotificationService.cfc:450` variable 'content' has no component ref |
| 2 | `contentitem → modules/contentbox/models/content/BaseContent.cfc` | 1 | high | declares setContentTemplate(), setChildContentTemplate() | `modules/contentbox/models/content/ContentTemplateService.cfc:109` variable 'contentItem' has no component ref |
| 2 | `contentobjects[] → modules/contentbox/models/content/BaseContent.cfc` | 1 | high | declares setpublishedDate(), setisPublished() | `modules/contentbox/models/content/ContentService.cfc:627` variable 'contentObjects[]' has no component ref |
| 2 | `e → modules/contentbox/models/system/Site.cfc` | 2 | low | declares hasCategories(), getCategories() (2 candidates) | `modules/contentbox/models/system/CBHelper.cfc:1839` variable 'e' has no component ref |
| 2 | `entry → modules/contentbox/models/content/BaseContent.cfc` | 1 | medium | declares getIsPublished() | `modules/contentbox/models/content/EntryService.cfc:32` variable 'entry' has no component ref |
| 2 | `field → modules/contentbox/models/content/CustomField.cfc` | 3 | high | declares getKey(), getValue() | `modules/contentbox/models/content/ContentTemplateService.cfc:194` variable 'field' has no component ref |
| 2 | `incomment → modules/contentbox/models/content/BaseContent.cfc` | 3 | low | declares getSiteSlug() (3 candidates) | `modules/contentbox/models/comments/CommentService.cfc:197` variable 'inComment' has no component ref |
| 2 | `instance.iofile` | 2 | none |  | `modules/contentbox/models/util/ZipUtil.cfc:126` variable 'instance.ioFile' has no component ref |
| 2 | `lastlogin` | 5 | none |  | `modules/contentbox/modules/contentbox-admin/views/dashboard/latestLogins.cfm:12` variable 'lastlogin' has no component ref |
| 2 | `menuitem → modules/contentbox/models/menu/item/MediaMenuItem.cfc` | 1 | high | declares setMediaPath(), setActive() | `modules/contentbox/modules/contentbox-admin/interceptors/MenuCleanup.cfc:163` variable 'menuItem' has no component ref |
| 2 | `menuitem → modules/contentbox/models/menu/item/SubMenuItem.cfc` | 1 | high | declares setMenuSlug(), setActive() | `modules/contentbox/modules/contentbox-admin/interceptors/MenuCleanup.cfc:122` variable 'menuItem' has no component ref |
| 2 | `newchild → modules/contentbox/models/content/BaseContent.cfc` | 9 | high | declares clone(), getSlug() | `modules/contentbox/models/content/BaseContent.cfc:1421` variable 'newChild' has no component ref |
| 2 | `newtemplate → modules/contentbox/models/content/ContentTemplate.cfc` | 1 | high | declares getSchema(), setDefinition() | `modules/contentbox/models/content/ContentTemplateService.cfc:176` variable 'newTemplate' has no component ref |
| 2 | `ocurrentcontent → modules/contentbox/models/system/Site.cfc` | 5 | low | declares getslug() (5 candidates) | `modules/contentbox/models/system/CBHelper.cfc:1055` variable 'oCurrentContent' has no component ref |
| 2 | `opermission → modules/contentbox/models/BaseEntityMethods.cfc` | 2 | low | declares isLoaded() (2 candidates) | `modules/contentbox/models/security/PermissionService.cfc:155` variable 'oPermission' has no component ref |
| 2 | `orule → modules/contentbox/models/BaseEntityMethods.cfc` | 2 | low | declares isLoaded() (2 candidates) | `modules/contentbox/models/security/SecurityRuleService.cfc:185` variable 'oRule' has no component ref |
| 2 | `osite → modules/contentbox/models/BaseEntityMethods.cfc` | 2 | low | declares isLoaded() (2 candidates) | `modules/contentbox/models/system/SiteService.cfc:392` variable 'oSite' has no component ref |
| 2 | `pageresults.content[] → modules/contentbox/models/content/BaseContent.cfc` | 2 | low | declares hasChild() (2 candidates) | `modules/contentbox/models/system/CBHelper.cfc:2536` variable 'pageResults.content[]' has no component ref |
| 2 | `prc.oblockbyip → modules/contentbox/models/security/LoginAttempt.cfc` | 1 | high | declares getAttempts(), setAttempts(), setCreatedDate() | `modules/contentbox/models/security/LoginTracker.cfc:135` variable 'prc.oBlockByIP' has no component ref |
| 2 | `prc.oblockbyusername → modules/contentbox/models/security/LoginAttempt.cfc` | 1 | high | declares getAttempts(), setAttempts(), setCreatedDate() | `modules/contentbox/models/security/LoginTracker.cfc:145` variable 'prc.oBlockByUsername' has no component ref |
| 2 | `prc.oentity → modules/contentbox/models/search/SearchResults.cfc` | 10 | medium | declares getMemento() | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/baseHandler.cfc:107` variable 'prc.oEntity' has no component ref |
| 2 | `prc.orelatedcontent → modules/contentbox/models/content/BaseContent.cfc` | 1 | medium | declares getContentID() | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/comments.cfc:57` variable 'prc.oRelatedContent' has no component ref |
| 2 | `relatedcontent[] → modules/contentbox/models/content/BaseContent.cfc` | 1 | medium | declares isContentPublished() | `modules/contentbox/widgets/RelatedContent.cfc:78` variable 'relatedContent[]' has no component ref |
| 2 | `securityinterceptor` | 2 | none |  | `modules/contentbox/modules/contentbox-admin/handlers/securityRules.cfc:48` variable 'securityInterceptor' has no component ref |
| 2 | `stat → modules/contentbox/models/content/BaseContent.cfc` | 1 | high | declares getContentID(), getTitle() | `modules/contentbox/models/content/ContentService.cfc:791` variable 'stat' has no component ref |
| 2 | `subscriber → modules/contentbox/models/subscriptions/Subscriber.cfc` | 1 | high | declares getSubscriberEmail(); named like the receiver 'subscriber' | `modules/contentbox/models/comments/CommentService.cfc:261` variable 'subscriber' has no component ref |
| 2 | `subscription → modules/contentbox/models/subscriptions/BaseSubscription.cfc` | 1 | high | declares getSubscriber(), getSubscriptionToken() | `modules/contentbox/models/comments/CommentService.cfc:258` variable 'subscription' has no component ref |
| 2 | `target → modules/contentbox/models/content/ContentTemplate.cfc` | 1 | high | declares isNameUniqueInSite(), isGlobalUniqueInSite() | `modules/contentbox/models/content/ContentTemplate.cfc:308` variable 'target' has no component ref |
| 2 | `targetsite → modules/contentbox/models/system/Site.cfc` | 5 | low | declares getSlug() (5 candidates) | `modules/contentbox/models/system/SettingService.cfc:314` variable 'targetSite' has no component ref |
| 2 | `themerecord.descriptor → modules/contentbox/themes/default/Theme.cfc` | 1 | medium | declares onDeactivation() | `modules/contentbox/models/ui/ThemeService.cfc:364` variable 'themeRecord.descriptor' has no component ref |
| 2 | `thisgroup → modules/contentbox/models/security/PermissionGroup.cfc` | 1 | high | declares getPermissionGroupID(), getName() | `modules/contentbox/modules/contentbox-admin/views/authors/new.cfm:132` variable 'thisGroup' has no component ref |
| 2 | `thisrole → modules/contentbox/models/security/Author.cfc` | 2 | low | declares getRole() (2 candidates) | `modules/contentbox/modules/contentbox-admin/views/securityRules/editor.cfm:173` variable 'thisRole' has no component ref |
| 2 | `topcommented → modules/contentbox/models/content/BaseContent.cfc` | 4 | high | declares getTitle(), getNumberOfComments() | `modules/contentbox/modules/contentbox-admin/views/dashboard/latestSnapshot.cfm:55` variable 'topCommented' has no component ref |
| 2 | `validationresult` | 0 | none |  | `modules/contentbox/models/validators/UniqueSiteFieldValidator.cfc:75` variable 'validationResult' has no component ref |
| 2 | `variables.cache` | 1 | none |  | `modules/contentbox/models/rss/RSSService.cfc:62` variable 'variables.cache' has no component ref |
| 2 | `variables.settingservice → modules/contentbox/models/system/SettingService.cfc` | 2 | medium | declares getAllSettings(); named like the receiver 'settingService' (2 candidates) | `modules/contentbox/models/rss/RSSService.cfc:326` variable 'variables.settingService' has no component ref |
| 2 | `variables[] → modules/contentbox/models/content/CategoryService.cfc` | 14 | low | declares getAllForExport() (14 candidates) | `modules/contentbox/models/exporters/ContentBoxExporter.cfc:235` variable 'variables[]' has no component ref |
| 2 | `version → modules/contentbox/models/content/ContentVersion.cfc` | 4 | low | declares getIsActive(), setIsActive() (4 candidates) | `modules/contentbox/models/content/BaseContent.cfc:831` variable 'version' has no component ref |
| 2 | `vresult` | 0 | none |  | `modules/contentbox/modules/contentbox-admin/handlers/authors.cfc:550` variable 'vResult' has no component ref |
| 2 | `widget → modules/contentbox/models/ui/Widget.cfc` | 2 | medium | declares getIcon(); named like the receiver 'widget' (2 candidates) | `modules/contentbox/models/ui/WidgetService.cfc:506` variable 'widget' has no component ref |
| 1 | `activecontent → modules/contentbox/models/security/Author.cfc` | 5 | medium | declares getInfoSnapshot(); named like the receiver 'Author' (5 candidates) | `modules/contentbox/models/content/BaseContent.cfc:661` variable 'activeContent' has no component ref |
| 1 | `application.cbcontroller → modules/contentbox/models/system/SettingService.cfc` | 3 | medium | declares getSetting() | `Application.cfc:156` variable 'application.cbController' has no component ref |
| 1 | `application.wirebox` | 4 | none |  | `config/modules/cbfs.cfc:21` variable 'application.wirebox' has no component ref |
| 1 | `args.ocontent → modules/contentbox/models/BaseEntityMethods.cfc` | 1 | medium | declares getDisplayCreatedDate() | `modules/contentbox/modules/contentbox-ui/views/adminbar/index.cfm:63` variable 'args.oContent' has no component ref |
| 1 | `arguments.comment → modules/contentbox/models/content/BaseContent.cfc` | 3 | low | declares getContentType() (2 candidates) | `modules/contentbox/models/system/CBHelper.cfc:1748` variable 'arguments.comment' has no component ref |
| 1 | `arguments.data.comment → modules/contentbox/models/comments/Comment.cfc` | 7 | medium | declares getRelatedContent(); named like the receiver 'comment' (7 candidates) | `modules/contentbox/models/rss/RSSCacheCleanup.cfc:40` variable 'arguments.data.comment' has no component ref |
| 1 | `arguments.data.comment → modules/contentbox/models/content/BaseContent.cfc` | 5 | low | declares getSlug() (5 candidates) | `modules/contentbox/models/rss/RSSCacheCleanup.cfc:40` variable 'arguments.data.comment' has no component ref |
| 1 | `arguments.field → modules/contentbox/models/search/SearchResults.cfc` | 10 | medium | declares getMemento() | `modules/contentbox/modules/contentbox-admin/views/_components/editor/CustomFieldsHelper.cfm:6` variable 'arguments.field' has no component ref |
| 1 | `arguments.group → modules/contentbox/models/security/PermissionGroup.cfc` | 1 | medium | declares addAuthor() | `modules/contentbox/models/security/Author.cfc:446` variable 'arguments.group' has no component ref |
| 1 | `arguments.item → modules/contentbox/models/content/Category.cfc` | 5 | low | declares getCategory() (4 candidates) | `modules/contentbox/models/content/BaseContent.cfc:1729` variable 'arguments.item' has no component ref |
| 1 | `arguments.menuitem → modules/contentbox/models/menu/item/BaseMenuItem.cfc` | 1 | medium | declares setMenu() | `modules/contentbox/models/menu/Menu.cfc:173` variable 'arguments.menuItem' has no component ref |
| 1 | `arguments.spec` | 0 | none |  | `tests/specs/contentbox-web/contentbox/unit/modules/ModuleServiceTest.cfc:34` variable 'arguments.spec' has no component ref |
| 1 | `arguments.target → modules/contentbox/models/content/CategoryService.cfc` | 3 | medium | declares isSlugUnique(); named like the receiver 'CategoryService' (3 candidates) | `modules/contentbox/models/content/Category.cfc:120` variable 'arguments.target' has no component ref |
| 1 | `arguments.target → modules/contentbox/models/content/ContentService.cfc` | 3 | medium | declares isSlugUnique(); named like the receiver 'ContentService' (3 candidates) | `modules/contentbox/models/content/BaseContent.cfc:568` variable 'arguments.target' has no component ref |
| 1 | `arguments.thiscategory → modules/contentbox/models/system/Site.cfc` | 5 | low | declares getSlug() (5 candidates) | `modules/contentbox/models/system/Site.cfc:539` variable 'arguments.thisCategory' has no component ref |
| 1 | `arguments.thisitem → modules/contentbox/models/menu/item/BaseMenuItem.cfc` | 3 | low | declares hasParent() (2 candidates) | `modules/contentbox/models/menu/Menu.cfc:209` variable 'arguments.thisItem' has no component ref |
| 1 | `arguments.thismodule → modules/contentbox/models/modules/Module.cfc` | 4 | low | declares getIsActive() (4 candidates) | `modules/contentbox/models/modules/ModuleService.cfc:440` variable 'arguments.thisModule' has no component ref |
| 1 | `arguments.thispage → modules/contentbox/models/content/BaseContent.cfc` | 3 | low | declares hasParent() (2 candidates) | `modules/contentbox/models/system/Site.cfc:558` variable 'arguments.thisPage' has no component ref |
| 1 | `c → modules/contentbox/models/util/ZipUtil.cfc` | 4 | medium | declares list() | `modules/contentbox/models/content/ContentService.cfc:590` variable 'c' has no component ref |
| 1 | `child → modules/contentbox/models/menu/item/BaseMenuItem.cfc` | 3 | low | declares setParent() (2 candidates) | `modules/contentbox/models/menu/Menu.cfc:276` variable 'child' has no component ref |
| 1 | `config → modules/contentbox/modules_user/Hello/ModuleConfig.cfc` | 2 | low | declares onDeactivate() (2 candidates) | `modules/contentbox/models/modules/ModuleService.cfc:237` variable 'config' has no component ref |
| 1 | `contentitem.getparent → modules/contentbox/models/content/BaseContent.cfc` | 1 | medium | declares getContentID() | `modules/contentbox/models/content/ContentTemplateService.cfc:217` variable 'contentItem.getParent' has no component ref |
| 1 | `e → modules/contentbox/models/security/LoginAttempt.cfc` | 1 | medium | declares setIsBlocked() | `modules/contentbox/models/security/LoginTrackerService.cfc:63` variable 'e' has no component ref |
| 1 | `entries[] → modules/contentbox/models/comments/Comment.cfc` | 6 | low | declares getAuthor() (6 candidates) | `tests/specs/contentbox-web/contentbox/unit/content/EntryServiceTest.cfc:65` variable 'entries[]' has no component ref |
| 1 | `entries[] → modules/contentbox/models/security/Author.cfc` | 1 | high | declares getAuthorID(); named like the receiver 'Author' | `tests/specs/contentbox-web/contentbox/unit/content/EntryServiceTest.cfc:65` variable 'entries[]' has no component ref |
| 1 | `entry → modules/contentbox/models/comments/Comment.cfc` | 4 | low | declares getContent() (4 candidates) | `modules/contentbox/models/system/NotificationService.cfc:190` variable 'entry' has no component ref |
| 1 | `entryresults.content[] → modules/contentbox/models/content/BaseContent.cfc` | 4 | low | declares getTitle() (4 candidates) | `modules/contentbox/widgets/RecentEntries.cfc:95` variable 'entryResults.content[]' has no component ref |
| 1 | `filesystemutil` | 0 | none |  | `tests/resources/seeds/archive/BaseSeeder.cfc:44` variable 'fileSystemUtil' has no component ref |
| 1 | `grammar` | 0 | none |  | `modules/contentbox/migrations/2020_08_24_150933_v5Upgrade.cfc:535` variable 'grammar' has no component ref |
| 1 | `item → modules/contentbox/models/menu/item/MediaMenuItem.cfc` | 1 | medium | declares setMediaPath() | `modules/contentbox/modules/contentbox-admin/interceptors/MenuCleanup.cfc:150` variable 'item' has no component ref |
| 1 | `item → modules/contentbox/models/menu/item/SubMenuItem.cfc` | 1 | medium | declares setMenuSlug() | `modules/contentbox/modules/contentbox-admin/interceptors/MenuCleanup.cfc:100` variable 'item' has no component ref |
| 1 | `item → modules/contentbox/models/menu/providers/ContentProvider.cfc` | 7 | low | declares getDisplayTemplate() (7 candidates) | `modules/contentbox/models/system/CBHelper.cfc:2404` variable 'item' has no component ref |
| 1 | `item → modules/contentbox/models/system/Site.cfc` | 5 | medium | declares getSlug(); named like the receiver 'Site' (5 candidates) | `modules/contentbox/models/system/SettingService.cfc:583` variable 'item' has no component ref |
| 1 | `module → modules/contentbox/models/modules/Module.cfc` | 4 | high | declares getName(), getTitle(); named like the receiver 'module' | `modules/contentbox/modules/contentbox-admin/views/tools/exporter.cfm:287` variable 'module' has no component ref |
| 1 | `ocategory → modules/contentbox/models/search/SearchResults.cfc` | 10 | medium | declares getMemento() | `modules/contentbox/modules/contentbox-admin/handlers/categories.cfc:114` variable 'oCategory' has no component ref |
| 1 | `oconfig → modules/contentbox/modules_user/Hello/ModuleConfig.cfc` | 2 | low | declares onActivate() (2 candidates) | `modules/contentbox/models/modules/ModuleService.cfc:286` variable 'oConfig' has no component ref |
| 1 | `originalservice → modules/contentbox/models/content/BaseContent.cfc` | 3 | low | declares setParent() (2 candidates) | `modules/contentbox/models/content/BaseContent.cfc:1412` variable 'originalService' has no component ref |
| 1 | `otheme → modules/contentbox/themes/default/Theme.cfc` | 1 | high | declares onActivation(); named like the receiver 'oTheme' | `modules/contentbox/models/ui/ThemeService.cfc:270` variable 'oTheme' has no component ref |
| 1 | `outputstream` | 0 | none |  | `modules/contentbox/models/media/ForwardMediaProvider.cfc:69` variable 'outputStream' has no component ref |
| 1 | `page → modules/contentbox/models/comments/Comment.cfc` | 4 | low | declares getContent() (4 candidates) | `modules/contentbox/models/system/NotificationService.cfc:320` variable 'page' has no component ref |
| 1 | `page → modules/contentbox/models/content/BaseContent.cfc` | 3 | low | declares renderContent() (2 candidates) | `modules/contentbox/widgets/PageInclude.cfc:39` variable 'page' has no component ref |
| 1 | `pages[] → modules/contentbox/models/comments/Comment.cfc` | 6 | low | declares getAuthor() (6 candidates) | `tests/specs/contentbox-web/contentbox/unit/content/PageServiceTest.cfc:45` variable 'pages[]' has no component ref |
| 1 | `pages[] → modules/contentbox/models/security/Author.cfc` | 1 | high | declares getAuthorID(); named like the receiver 'Author' | `tests/specs/contentbox-web/contentbox/unit/content/PageServiceTest.cfc:45` variable 'pages[]' has no component ref |
| 1 | `prc` | 0 | none |  | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/baseContentHandler.cfc:140` variable 'prc' has no component ref |
| 1 | `prc → modules/contentbox/models/security/Author.cfc` | 3 | low | declares hasPermission() (3 candidates) | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/baseContentHandler.cfc:140` variable 'prc' has no component ref |
| 1 | `prc.activedisk → modules/contentbox/modules/contentbox-admin/modules/contentbox-filebrowser/handlers/Home.cfc` | 1 | medium | declares download() | `modules/contentbox/modules/contentbox-admin/modules/contentbox-filebrowser/handlers/Home.cfc:351` variable 'prc.activeDisk' has no component ref |
| 1 | `prc.categories[] → modules/contentbox/models/content/Category.cfc` | 1 | medium | declares getCategoryID() | `modules/contentbox/modules/contentbox-admin/views/_components/editor/sidebar/Categories.cfm:20` variable 'prc.categories[]' has no component ref |
| 1 | `prc.owidget → modules/contentbox/models/ui/BaseWidget.cfc` | 1 | medium | declares getPublicMethods() | `modules/contentbox/modules/contentbox-admin/handlers/widgets.cfc:52` variable 'prc.oWidget' has no component ref |
| 1 | `prc.page → modules/contentbox/models/BaseEntityMethods.cfc` | 2 | low | declares isLoaded() (2 candidates) | `modules/contentbox/models/system/CBHelper.cfc:2457` variable 'prc.page' has no component ref |
| 1 | `prc.page → modules/contentbox/models/content/Page.cfc` | 2 | medium | declares isHomePage(); named like the receiver 'page' (2 candidates) | `modules/contentbox/models/system/CBHelper.cfc:753` variable 'prc.page' has no component ref |
| 1 | `prc.provider → modules/contentbox/models/BaseEntityMethods.cfc` | 5 | low | declares getEntityName() (3 candidates) | `modules/contentbox/modules/contentbox-admin/handlers/menus.cfc:145` variable 'prc.provider' has no component ref |
| 1 | `prc.site → modules/contentbox/models/system/Site.cfc` | 1 | high | declares getDomainAliasesAsJSON(); named like the receiver 'site' | `modules/contentbox/modules/contentbox-admin/views/sites/editorHelper.cfm:12` variable 'prc.site' has no component ref |
| 1 | `r → modules/contentbox/models/content/BaseContent.cfc` | 7 | low | declares getSite() (7 candidates) | `tests/specs/contentbox-web/contentbox/unit/content/RelocationTest.cfc:53` variable 'r' has no component ref |
| 1 | `r → modules/contentbox/models/system/Site.cfc` | 5 | medium | declares getSiteID(); named like the receiver 'Site' (5 candidates) | `tests/specs/contentbox-web/contentbox/unit/content/RelocationTest.cfc:53` variable 'r' has no component ref |
| 1 | `rc.comment → modules/contentbox/models/content/BaseContent.cfc` | 4 | low | declares getTitle() (4 candidates) | `modules/contentbox/modules/contentbox-admin/views/comments/moderate.cfm:44` variable 'rc.comment' has no component ref |
| 1 | `results.author → modules/contentbox/models/security/Author.cfc` | 2 | medium | declares getPassword(); named like the receiver 'author' (2 candidates) | `modules/contentbox/modules/contentbox-admin/modules/contentbox-security/handlers/security.cfc:307` variable 'results.author' has no component ref |
| 1 | `setting → modules/contentbox/models/system/Site.cfc` | 5 | medium | declares getSlug(); named like the receiver 'Site' (5 candidates) | `modules/contentbox/modules/contentbox-admin/views/settings/rawSettingsTable.cfm:54` variable 'setting' has no component ref |
| 1 | `setup → modules/contentbox-installer/models/Setup.cfc` | 1 | high | declares getUserData(); named like the receiver 'setup' | `modules/contentbox-installer/models/InstallerService.cfc:182` variable 'setup' has no component ref |
| 1 | `template` | 0 | none |  | `modules/contentbox/models/content/BaseContent.cfc:1820` variable 'template' has no component ref |
| 1 | `template → modules/contentbox/models/search/SearchResults.cfc` | 10 | medium | declares getMemento() | `modules/contentbox/modules/contentbox-admin/handlers/contentTemplates.cfc:65` variable 'template' has no component ref |
| 1 | `testsite → modules/contentbox/models/content/BaseContent.cfc` | 5 | low | declares getSiteID() (5 candidates) | `tests/specs/contentbox-api/SitesSpec.cfc:46` variable 'testSite' has no component ref |
| 1 | `thiscategory → modules/contentbox/models/search/SearchResults.cfc` | 10 | medium | declares getMemento() | `modules/contentbox/modules/contentbox-admin/handlers/categories.cfc:68` variable 'thisCategory' has no component ref |
| 1 | `thiscomment → modules/contentbox/models/comments/Comment.cfc` | 1 | medium | declares setIsApproved() | `modules/contentbox/models/comments/CommentService.cfc:376` variable 'thisComment' has no component ref |
| 1 | `thiscontent → modules/contentbox/models/BaseEntityMethods.cfc` | 1 | medium | declares getDisplayCreatedDate() | `modules/contentbox/modules/contentbox-admin/views/content/contentViewlet.cfm:91` variable 'thisContent' has no component ref |
| 1 | `variables.activecontent → modules/contentbox/models/content/BaseContent.cfc` | 4 | low | declares getContent() (4 candidates) | `modules/contentbox/models/content/BaseContent.cfc:1148` variable 'variables.activeContent' has no component ref |
| 1 | `variables.contentversions[] → modules/contentbox/models/content/ContentVersion.cfc` | 8 | low | declares getVersion() (4 candidates) | `modules/contentbox/models/content/BaseContent.cfc:847` variable 'variables.contentVersions[]' has no component ref |
| 1 | `variables.editors[] → modules/contentbox/models/ui/editors/IEditor.cfc` | 8 | low | declares getDisplayName() (8 candidates) | `modules/contentbox/models/ui/editors/EditorService.cfc:128` variable 'variables.editors[]' has no component ref |
| 1 | `variables.logger` | 3 | none |  | `modules/contentbox/models/security/AuthorService.cfc:205` variable 'variables.logger' has no component ref |
| 1 | `variables.providers[] → modules/contentbox/models/security/twofactor/ITwoFactorProvider.cfc` | 8 | low | declares getDisplayName() (8 candidates) | `modules/contentbox/models/security/twofactor/TwoFactorService.cfc:114` variable 'variables.providers[]' has no component ref |
| 1 | `variables.siteservice` | 2 | none |  | `tests/resources/BaseApiTest.cfc:164` variable 'variables.siteService' has no component ref |
| 1 | `ziptemp` | 0 | none |  | `modules/contentbox/models/util/ZipUtil.cfc:253` variable 'zipTemp' has no component ref |

<details><summary>Groups with several candidates</summary>

- `arguments.site → modules/contentbox/models/system/Site.cfc` — 19 finding(s), 5 candidate(s):
  - medium `modules/contentbox/models/system/Site.cfc` — declares getsiteID(); named like the receiver 'site'
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getsiteID()
  - low `modules/contentbox/models/content/Category.cfc` — declares getsiteID()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getsiteID()
  - low `modules/contentbox/models/system/Setting.cfc` — declares getsiteID()
- `arguments.page → modules/contentbox/models/content/BaseContent.cfc` — 14 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSlug()
  - low `modules/contentbox/models/system/Site.cfc` — declares getSlug()
- `comment → modules/contentbox/models/comments/Comment.cfc` — 13 finding(s), 3 candidate(s):
  - medium `modules/contentbox/models/comments/Comment.cfc` — declares getAuthorEmail(), getRelatedContent(); named like the receiver 'comment'
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getAuthorEmail(), getRelatedContent()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getAuthorEmail(), getRelatedContent()
- `arguments.category → modules/contentbox/models/content/Category.cfc` — 8 finding(s), 2 candidate(s):
  - medium `modules/contentbox/models/content/Category.cfc` — declares getSlug(), getCategory(); named like the receiver 'category'
  - low `modules/contentbox/models/system/Site.cfc` — declares getSlug(), getCategory()
- `arguments.comment → modules/contentbox/models/comments/Comment.cfc` — 8 finding(s), 2 candidate(s):
  - medium `modules/contentbox/models/comments/Comment.cfc` — declares getRelatedContent(), getAuthorEmail(), getMemento(), getParentTitle(); named like the receiver 'comment'
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getRelatedContent(), getAuthorEmail(), getMemento(), getParentTitle()
- `settingservice → modules/contentbox/models/system/SettingService.cfc` — 7 finding(s), 2 candidate(s):
  - medium `modules/contentbox/models/system/SettingService.cfc` — declares getAllSettings(); named like the receiver 'settingService'
  - low `modules/contentbox/models/security/twofactor/BaseTwoFactorProvider.cfc` — declares getAllSettings()
- `arguments.thiscontent → modules/contentbox/models/content/BaseContent.cfc` — 6 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares hasParent()
  - low `modules/contentbox/models/menu/item/BaseMenuItem.cfc` — declares hasParent()
- `arguments.thisitem → modules/contentbox/models/content/BaseContent.cfc` — 6 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getInfoSnapshot()
  - low `modules/contentbox/models/security/Author.cfc` — declares getInfoSnapshot()
  - low `modules/contentbox/models/subscriptions/Subscriber.cfc` — declares getInfoSnapshot()
  - low `modules/contentbox/models/system/Site.cfc` — declares getInfoSnapshot()
  - low `modules/contentbox/models/menu/item/BaseMenuItem.cfc` — declares getInfoSnapshot()
- `owidget → modules/contentbox/models/ui/Widget.cfc` — 6 finding(s), 2 candidate(s):
  - medium `modules/contentbox/models/ui/Widget.cfc` — declares getAuthorURL(), getName(), getDescription(), getVersion(), getAuthor(); named like the receiver 'oWidget'
  - low `modules/contentbox/models/ui/BaseWidget.cfc` — declares getAuthorURL(), getName(), getDescription(), getVersion(), getAuthor()
- `entry → modules/contentbox/models/content/Entry.cfc` — 5 finding(s), 2 candidate(s):
  - medium `modules/contentbox/models/content/Entry.cfc` — declares getSite(), getTitle(), hasExcerpt(), renderExcerpt(), renderContent(); named like the receiver 'entry'
  - low `modules/contentbox/models/content/Page.cfc` — declares getSite(), getTitle(), hasExcerpt(), renderExcerpt(), renderContent()
- `page → modules/contentbox/models/content/Page.cfc` — 5 finding(s), 2 candidate(s):
  - medium `modules/contentbox/models/content/Page.cfc` — declares getSite(), getTitle(), hasExcerpt(), renderExcerpt(), renderContent(); named like the receiver 'page'
  - low `modules/contentbox/models/content/Entry.cfc` — declares getSite(), getTitle(), hasExcerpt(), renderExcerpt(), renderContent()
- `arguments.categories[] → modules/contentbox/models/content/Category.cfc` — 4 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/content/Category.cfc` — declares getCategory(), getNumberOfEntries()
  - low `modules/contentbox/models/system/Site.cfc` — declares getCategory(), getNumberOfEntries()
- `arguments.thissite → modules/contentbox/models/content/BaseContent.cfc` — 4 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSlug()
  - low `modules/contentbox/models/system/Site.cfc` — declares getSlug()
- `prc.opaging → modules/contentbox/models/ui/Paging.cfc` — 4 finding(s), 25 candidate(s):
  - medium `modules/contentbox/models/ui/Paging.cfc` — declares renderit(); named like the receiver 'oPaging'
  - low `modules/contentbox/models/ui/BaseWidget.cfc` — declares renderit()
  - low `modules/contentbox/widgets/Archives.cfc` — declares renderit()
  - low `modules/contentbox/widgets/Categories.cfc` — declares renderit()
  - low `modules/contentbox/widgets/CommentForm.cfc` — declares renderit()
  - low `modules/contentbox/widgets/ContentStore.cfc` — declares renderit()
  - low `modules/contentbox/widgets/EntryInclude.cfc` — declares renderit()
  - low `modules/contentbox/widgets/Menu.cfc` — declares renderit()
  - … 17 more
- `arguments.entry → modules/contentbox/models/system/Site.cfc` — 3 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/system/Site.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSlug()
- `arguments.page → modules/contentbox/models/system/Site.cfc` — 3 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/system/Site.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSlug()
- `arguments.thischild → modules/contentbox/models/content/BaseContent.cfc` — 3 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getTitle(), getSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getTitle(), getSlug()
- `commentresults.comments[] → modules/contentbox/models/comments/Comment.cfc` — 3 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/comments/Comment.cfc` — declares getParentTitle()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getParentTitle()
- `menu → modules/contentbox/models/menu/Menu.cfc` — 3 finding(s), 2 candidate(s):
  - medium `modules/contentbox/models/menu/Menu.cfc` — declares getSlug(), getTitle(); named like the receiver 'menu'
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug(), getTitle()
- `prc.activedisk → modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/authors.cfc` — 3 finding(s), 11 candidate(s):
  - low `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/authors.cfc` — declares create()
  - low `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/baseHandler.cfc` — declares create()
  - low `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/categories.cfc` — declares create()
  - low `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/comments.cfc` — declares create()
  - low `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/contentStore.cfc` — declares create()
  - low `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/contentTemplates.cfc` — declares create()
  - low `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/entries.cfc` — declares create()
  - low `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/menus.cfc` — declares create()
  - … 3 more
- `prc.oeditordriver → modules/contentbox/modules/contentbox-admin/modules/contentbox-ckeditor/models/CKEditor.cfc` — 3 finding(s), 4 candidate(s):
  - low `modules/contentbox/modules/contentbox-admin/modules/contentbox-ckeditor/models/CKEditor.cfc` — declares startup(), shutdown(), loadAssets()
  - low `modules/contentbox/modules/contentbox-admin/modules/contentbox-markdowneditor/models/MarkdownEditor.cfc` — declares startup(), shutdown(), loadAssets()
  - low `modules/contentbox/models/ui/editors/IEditor.cfc` — declares startup(), shutdown(), loadAssets()
  - low `tests/specs/contentbox-web/contentbox/unit/ui/editors/MockEditor.cfc` — declares startup(), shutdown(), loadAssets()
- `prc.owidget → modules/contentbox/models/modules/Module.cfc` — 3 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/modules/Module.cfc` — declares getName(), getVersion(), getForgeBoxSlug(), getDescription()
  - low `modules/contentbox/models/ui/BaseWidget.cfc` — declares getName(), getVersion(), getForgeBoxSlug(), getDescription()
- `thispage → modules/contentbox/models/content/BaseContent.cfc` — 3 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug(), setSlug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSlug(), setSlug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSlug(), setSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSlug(), setSlug()
  - low `modules/contentbox/models/system/Site.cfc` — declares getSlug(), setSlug()
- `arguments.data.menu → modules/contentbox/models/menu/Menu.cfc` — 2 finding(s), 5 candidate(s):
  - medium `modules/contentbox/models/menu/Menu.cfc` — declares getSlug(); named like the receiver 'menu'
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSlug()
  - low `modules/contentbox/models/system/Site.cfc` — declares getSlug()
- `arguments.menuitem → modules/contentbox/models/menu/Menu.cfc` — 2 finding(s), 5 candidate(s):
  - medium `modules/contentbox/models/menu/Menu.cfc` — declares getSiteID(); named like the receiver 'Menu'
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSiteID()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSiteID()
  - low `modules/contentbox/models/system/Setting.cfc` — declares getSiteID()
  - low `modules/contentbox/models/system/Site.cfc` — declares getSiteID()
- `arguments.original → modules/contentbox/models/content/Entry.cfc` — 2 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/content/Entry.cfc` — declares hasExcerpt(), getExcerpt()
  - low `modules/contentbox/models/content/Page.cfc` — declares hasExcerpt(), getExcerpt()
- `arguments.originalservice → modules/contentbox/models/content/CategoryService.cfc` — 2 finding(s), 23 candidate(s):
  - low `modules/contentbox/models/content/CategoryService.cfc` — declares save()
  - low `modules/contentbox/models/content/ContentStoreService.cfc` — declares save()
  - low `modules/contentbox/models/content/EntryService.cfc` — declares save()
  - low `modules/contentbox/models/content/PageService.cfc` — declares save()
  - low `modules/contentbox/models/menu/MenuService.cfc` — declares save()
  - low `modules/contentbox/models/security/AuthorService.cfc` — declares save()
  - low `modules/contentbox/models/system/SiteService.cfc` — declares save()
  - low `modules/contentbox/modules/contentbox-admin/handlers/authors.cfc` — declares save()
  - … 15 more
- `arguments.relatedcontent[] → modules/contentbox/models/content/BaseContent.cfc` — 2 finding(s), 4 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getTitle()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getTitle()
  - low `modules/contentbox/models/modules/Module.cfc` — declares getTitle()
  - low `modules/contentbox/models/menu/item/BaseMenuItem.cfc` — declares getTitle()
- `arguments.target → modules/contentbox/models/system/Site.cfc` — 2 finding(s), 5 candidate(s):
  - medium `modules/contentbox/models/system/Site.cfc` — declares getSiteID(); named like the receiver 'Site'
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSiteID()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSiteID()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSiteID()
  - low `modules/contentbox/models/system/Setting.cfc` — declares getSiteID()
- `cats[] → modules/contentbox/models/system/Site.cfc` — 2 finding(s), 4 candidate(s):
  - low `modules/contentbox/models/system/Site.cfc` — declares getCategory()
  - low `modules/contentbox/models/content/Category.cfc` — declares getCategory()
  - low `modules/contentbox/models/ui/BaseWidget.cfc` — declares getCategory()
  - low `modules/contentbox/models/ui/Widget.cfc` — declares getCategory()
- `content → modules/contentbox/models/comments/Comment.cfc` — 2 finding(s), 4 candidate(s):
  - low `modules/contentbox/models/comments/Comment.cfc` — declares getContent()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getContent()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getContent()
  - low `modules/contentbox/models/exporters/DataExporter.cfc` — declares getContent()
- `e → modules/contentbox/models/system/Site.cfc` — 2 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/system/Site.cfc` — declares hasCategories(), getCategories()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares hasCategories(), getCategories()
- `incomment → modules/contentbox/models/content/BaseContent.cfc` — 2 finding(s), 3 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSiteSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSiteSlug()
  - low `modules/contentbox-installer/models/Setup.cfc` — declares getSiteSlug()
- `ocurrentcontent → modules/contentbox/models/system/Site.cfc` — 2 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/system/Site.cfc` — declares getslug()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getslug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getslug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getslug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getslug()
- `opermission → modules/contentbox/models/BaseEntityMethods.cfc` — 2 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/BaseEntityMethods.cfc` — declares isLoaded()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares isLoaded()
- `orule → modules/contentbox/models/BaseEntityMethods.cfc` — 2 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/BaseEntityMethods.cfc` — declares isLoaded()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares isLoaded()
- `osite → modules/contentbox/models/BaseEntityMethods.cfc` — 2 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/BaseEntityMethods.cfc` — declares isLoaded()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares isLoaded()
- `pageresults.content[] → modules/contentbox/models/content/BaseContent.cfc` — 2 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares hasChild()
  - low `modules/contentbox/models/menu/item/BaseMenuItem.cfc` — declares hasChild()
- `targetsite → modules/contentbox/models/system/Site.cfc` — 2 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/system/Site.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSlug()
- `thisrole → modules/contentbox/models/security/Author.cfc` — 2 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/security/Author.cfc` — declares getRole()
  - low `modules/contentbox/models/security/Role.cfc` — declares getRole()
- `variables.settingservice → modules/contentbox/models/system/SettingService.cfc` — 2 finding(s), 2 candidate(s):
  - medium `modules/contentbox/models/system/SettingService.cfc` — declares getAllSettings(); named like the receiver 'settingService'
  - low `modules/contentbox/models/security/twofactor/BaseTwoFactorProvider.cfc` — declares getAllSettings()
- `variables[] → modules/contentbox/models/content/CategoryService.cfc` — 2 finding(s), 14 candidate(s):
  - low `modules/contentbox/models/content/CategoryService.cfc` — declares getAllForExport()
  - low `modules/contentbox/models/content/ContentService.cfc` — declares getAllForExport()
  - low `modules/contentbox/models/content/ContentStoreService.cfc` — declares getAllForExport()
  - low `modules/contentbox/models/content/ContentTemplateService.cfc` — declares getAllForExport()
  - low `modules/contentbox/models/content/EntryService.cfc` — declares getAllForExport()
  - low `modules/contentbox/models/content/PageService.cfc` — declares getAllForExport()
  - low `modules/contentbox/models/menu/MenuService.cfc` — declares getAllForExport()
  - low `modules/contentbox/models/security/AuthorService.cfc` — declares getAllForExport()
  - … 6 more
- `version → modules/contentbox/models/content/ContentVersion.cfc` — 2 finding(s), 4 candidate(s):
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getIsActive(), setIsActive()
  - low `modules/contentbox/models/modules/Module.cfc` — declares getIsActive(), setIsActive()
  - low `modules/contentbox/models/security/Author.cfc` — declares getIsActive(), setIsActive()
  - low `modules/contentbox/models/system/Site.cfc` — declares getIsActive(), setIsActive()
- `widget → modules/contentbox/models/ui/Widget.cfc` — 2 finding(s), 2 candidate(s):
  - medium `modules/contentbox/models/ui/Widget.cfc` — declares getIcon(); named like the receiver 'widget'
  - low `modules/contentbox/models/ui/BaseWidget.cfc` — declares getIcon()
- `activecontent → modules/contentbox/models/security/Author.cfc` — 1 finding(s), 5 candidate(s):
  - medium `modules/contentbox/models/security/Author.cfc` — declares getInfoSnapshot(); named like the receiver 'Author'
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getInfoSnapshot()
  - low `modules/contentbox/models/subscriptions/Subscriber.cfc` — declares getInfoSnapshot()
  - low `modules/contentbox/models/system/Site.cfc` — declares getInfoSnapshot()
  - low `modules/contentbox/models/menu/item/BaseMenuItem.cfc` — declares getInfoSnapshot()
- `arguments.comment → modules/contentbox/models/content/BaseContent.cfc` — 1 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getContentType()
  - low `modules/contentbox/models/content/ContentTemplate.cfc` — declares getContentType()
- `arguments.data.comment → modules/contentbox/models/comments/Comment.cfc` — 1 finding(s), 7 candidate(s):
  - medium `modules/contentbox/models/comments/Comment.cfc` — declares getRelatedContent(); named like the receiver 'comment'
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getRelatedContent()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getRelatedContent()
  - low `modules/contentbox/models/content/CustomField.cfc` — declares getRelatedContent()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getRelatedContent()
  - low `modules/contentbox/models/content/Stats.cfc` — declares getRelatedContent()
  - low `modules/contentbox/models/subscriptions/CommentSubscription.cfc` — declares getRelatedContent()
- `arguments.data.comment → modules/contentbox/models/content/BaseContent.cfc` — 1 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSlug()
  - low `modules/contentbox/models/system/Site.cfc` — declares getSlug()
- `arguments.item → modules/contentbox/models/content/Category.cfc` — 1 finding(s), 4 candidate(s):
  - low `modules/contentbox/models/content/Category.cfc` — declares getCategory()
  - low `modules/contentbox/models/system/Site.cfc` — declares getCategory()
  - low `modules/contentbox/models/ui/BaseWidget.cfc` — declares getCategory()
  - low `modules/contentbox/models/ui/Widget.cfc` — declares getCategory()
- `arguments.target → modules/contentbox/models/content/CategoryService.cfc` — 1 finding(s), 3 candidate(s):
  - medium `modules/contentbox/models/content/CategoryService.cfc` — declares isSlugUnique(); named like the receiver 'CategoryService'
  - low `modules/contentbox/models/content/ContentService.cfc` — declares isSlugUnique()
  - low `modules/contentbox/models/menu/MenuService.cfc` — declares isSlugUnique()
- `arguments.target → modules/contentbox/models/content/ContentService.cfc` — 1 finding(s), 3 candidate(s):
  - medium `modules/contentbox/models/content/ContentService.cfc` — declares isSlugUnique(); named like the receiver 'ContentService'
  - low `modules/contentbox/models/content/CategoryService.cfc` — declares isSlugUnique()
  - low `modules/contentbox/models/menu/MenuService.cfc` — declares isSlugUnique()
- `arguments.thiscategory → modules/contentbox/models/system/Site.cfc` — 1 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/system/Site.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSlug()
- `arguments.thisitem → modules/contentbox/models/menu/item/BaseMenuItem.cfc` — 1 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/menu/item/BaseMenuItem.cfc` — declares hasParent()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares hasParent()
- `arguments.thismodule → modules/contentbox/models/modules/Module.cfc` — 1 finding(s), 4 candidate(s):
  - low `modules/contentbox/models/modules/Module.cfc` — declares getIsActive()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getIsActive()
  - low `modules/contentbox/models/security/Author.cfc` — declares getIsActive()
  - low `modules/contentbox/models/system/Site.cfc` — declares getIsActive()
- `arguments.thispage → modules/contentbox/models/content/BaseContent.cfc` — 1 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares hasParent()
  - low `modules/contentbox/models/menu/item/BaseMenuItem.cfc` — declares hasParent()
- `child → modules/contentbox/models/menu/item/BaseMenuItem.cfc` — 1 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/menu/item/BaseMenuItem.cfc` — declares setParent()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares setParent()
- `config → modules/contentbox/modules_user/Hello/ModuleConfig.cfc` — 1 finding(s), 2 candidate(s):
  - low `modules/contentbox/modules_user/Hello/ModuleConfig.cfc` — declares onDeactivate()
  - low `modules/contentbox/modules_user/Hello/modules/hello-child/ModuleConfig.cfc` — declares onDeactivate()
- `entries[] → modules/contentbox/models/comments/Comment.cfc` — 1 finding(s), 6 candidate(s):
  - low `modules/contentbox/models/comments/Comment.cfc` — declares getAuthor()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getAuthor()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getAuthor()
  - low `modules/contentbox/models/modules/Module.cfc` — declares getAuthor()
  - low `modules/contentbox/models/ui/BaseWidget.cfc` — declares getAuthor()
  - low `modules/contentbox/models/ui/Widget.cfc` — declares getAuthor()
- `entry → modules/contentbox/models/comments/Comment.cfc` — 1 finding(s), 4 candidate(s):
  - low `modules/contentbox/models/comments/Comment.cfc` — declares getContent()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getContent()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getContent()
  - low `modules/contentbox/models/exporters/DataExporter.cfc` — declares getContent()
- `entryresults.content[] → modules/contentbox/models/content/BaseContent.cfc` — 1 finding(s), 4 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getTitle()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getTitle()
  - low `modules/contentbox/models/modules/Module.cfc` — declares getTitle()
  - low `modules/contentbox/models/menu/item/BaseMenuItem.cfc` — declares getTitle()
- `item → modules/contentbox/models/menu/providers/ContentProvider.cfc` — 1 finding(s), 7 candidate(s):
  - low `modules/contentbox/models/menu/providers/ContentProvider.cfc` — declares getDisplayTemplate()
  - low `modules/contentbox/models/menu/providers/FreeProvider.cfc` — declares getDisplayTemplate()
  - low `modules/contentbox/models/menu/providers/IMenuItemProvider.cfc` — declares getDisplayTemplate()
  - low `modules/contentbox/models/menu/providers/JSProvider.cfc` — declares getDisplayTemplate()
  - low `modules/contentbox/models/menu/providers/MediaProvider.cfc` — declares getDisplayTemplate()
  - low `modules/contentbox/models/menu/providers/SubMenuProvider.cfc` — declares getDisplayTemplate()
  - low `modules/contentbox/models/menu/providers/URLProvider.cfc` — declares getDisplayTemplate()
- `item → modules/contentbox/models/system/Site.cfc` — 1 finding(s), 5 candidate(s):
  - medium `modules/contentbox/models/system/Site.cfc` — declares getSlug(); named like the receiver 'Site'
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSlug()
- `oconfig → modules/contentbox/modules_user/Hello/ModuleConfig.cfc` — 1 finding(s), 2 candidate(s):
  - low `modules/contentbox/modules_user/Hello/ModuleConfig.cfc` — declares onActivate()
  - low `modules/contentbox/modules_user/Hello/modules/hello-child/ModuleConfig.cfc` — declares onActivate()
- `originalservice → modules/contentbox/models/content/BaseContent.cfc` — 1 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares setParent()
  - low `modules/contentbox/models/menu/item/BaseMenuItem.cfc` — declares setParent()
- `page → modules/contentbox/models/comments/Comment.cfc` — 1 finding(s), 4 candidate(s):
  - low `modules/contentbox/models/comments/Comment.cfc` — declares getContent()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getContent()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getContent()
  - low `modules/contentbox/models/exporters/DataExporter.cfc` — declares getContent()
- `page → modules/contentbox/models/content/BaseContent.cfc` — 1 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares renderContent()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares renderContent()
- `pages[] → modules/contentbox/models/comments/Comment.cfc` — 1 finding(s), 6 candidate(s):
  - low `modules/contentbox/models/comments/Comment.cfc` — declares getAuthor()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getAuthor()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getAuthor()
  - low `modules/contentbox/models/modules/Module.cfc` — declares getAuthor()
  - low `modules/contentbox/models/ui/BaseWidget.cfc` — declares getAuthor()
  - low `modules/contentbox/models/ui/Widget.cfc` — declares getAuthor()
- `prc → modules/contentbox/models/security/Author.cfc` — 1 finding(s), 3 candidate(s):
  - low `modules/contentbox/models/security/Author.cfc` — declares hasPermission()
  - low `modules/contentbox/models/security/PermissionGroup.cfc` — declares hasPermission()
  - low `modules/contentbox/models/security/Role.cfc` — declares hasPermission()
- `prc.page → modules/contentbox/models/BaseEntityMethods.cfc` — 1 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/BaseEntityMethods.cfc` — declares isLoaded()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares isLoaded()
- `prc.page → modules/contentbox/models/content/Page.cfc` — 1 finding(s), 2 candidate(s):
  - medium `modules/contentbox/models/content/Page.cfc` — declares isHomePage(); named like the receiver 'page'
  - low `modules/contentbox/models/system/CBHelper.cfc` — declares isHomePage()
- `prc.provider → modules/contentbox/models/BaseEntityMethods.cfc` — 1 finding(s), 3 candidate(s):
  - low `modules/contentbox/models/BaseEntityMethods.cfc` — declares getEntityName()
  - low `modules/contentbox/models/menu/providers/BaseProvider.cfc` — declares getEntityName()
  - low `modules/contentbox/models/menu/providers/IMenuItemProvider.cfc` — declares getEntityName()
- `r → modules/contentbox/models/content/BaseContent.cfc` — 1 finding(s), 7 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSite()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSite()
  - low `modules/contentbox/models/content/ContentTemplate.cfc` — declares getSite()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSite()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSite()
  - low `modules/contentbox/models/system/Setting.cfc` — declares getSite()
  - low `modules/contentbox/models/ui/BaseWidget.cfc` — declares getSite()
- `r → modules/contentbox/models/system/Site.cfc` — 1 finding(s), 5 candidate(s):
  - medium `modules/contentbox/models/system/Site.cfc` — declares getSiteID(); named like the receiver 'Site'
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSiteID()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSiteID()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSiteID()
  - low `modules/contentbox/models/system/Setting.cfc` — declares getSiteID()
- `rc.comment → modules/contentbox/models/content/BaseContent.cfc` — 1 finding(s), 4 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getTitle()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getTitle()
  - low `modules/contentbox/models/modules/Module.cfc` — declares getTitle()
  - low `modules/contentbox/models/menu/item/BaseMenuItem.cfc` — declares getTitle()
- `results.author → modules/contentbox/models/security/Author.cfc` — 1 finding(s), 2 candidate(s):
  - medium `modules/contentbox/models/security/Author.cfc` — declares getPassword(); named like the receiver 'author'
  - low `modules/contentbox-installer/models/Setup.cfc` — declares getPassword()
- `setting → modules/contentbox/models/system/Site.cfc` — 1 finding(s), 5 candidate(s):
  - medium `modules/contentbox/models/system/Site.cfc` — declares getSlug(); named like the receiver 'Site'
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSlug()
- `testsite → modules/contentbox/models/content/BaseContent.cfc` — 1 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSiteID()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSiteID()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSiteID()
  - low `modules/contentbox/models/system/Setting.cfc` — declares getSiteID()
  - low `modules/contentbox/models/system/Site.cfc` — declares getSiteID()
- `variables.activecontent → modules/contentbox/models/content/BaseContent.cfc` — 1 finding(s), 4 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getContent()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getContent()
  - low `modules/contentbox/models/comments/Comment.cfc` — declares getContent()
  - low `modules/contentbox/models/exporters/DataExporter.cfc` — declares getContent()
- `variables.contentversions[] → modules/contentbox/models/content/ContentVersion.cfc` — 1 finding(s), 4 candidate(s):
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getVersion()
  - low `modules/contentbox/models/modules/Module.cfc` — declares getVersion()
  - low `modules/contentbox/models/ui/BaseWidget.cfc` — declares getVersion()
  - low `modules/contentbox/models/ui/Widget.cfc` — declares getVersion()
- `variables.editors[] → modules/contentbox/models/ui/editors/IEditor.cfc` — 1 finding(s), 8 candidate(s):
  - low `modules/contentbox/models/ui/editors/IEditor.cfc` — declares getDisplayName()
  - low `modules/contentbox/models/exporters/BaseExporter.cfc` — declares getDisplayName()
  - low `modules/contentbox/models/media/BaseProvider.cfc` — declares getDisplayName()
  - low `modules/contentbox/models/security/twofactor/ITwoFactorProvider.cfc` — declares getDisplayName()
  - low `modules/contentbox/modules/contentbox-admin/modules/contentbox-ckeditor/models/CKEditor.cfc` — declares getDisplayName()
  - low `modules/contentbox/modules/contentbox-admin/modules/contentbox-markdowneditor/models/MarkdownEditor.cfc` — declares getDisplayName()
  - low `modules/contentbox/modules/contentbox-admin/modules/contentbox-security/modules/contentbox-email-twofactor/models/EmailTwoFactorProvider.cfc` — declares getDisplayName()
  - low `tests/specs/contentbox-web/contentbox/unit/ui/editors/MockEditor.cfc` — declares getDisplayName()
- `variables.providers[] → modules/contentbox/models/security/twofactor/ITwoFactorProvider.cfc` — 1 finding(s), 8 candidate(s):
  - low `modules/contentbox/models/security/twofactor/ITwoFactorProvider.cfc` — declares getDisplayName()
  - low `modules/contentbox/models/exporters/BaseExporter.cfc` — declares getDisplayName()
  - low `modules/contentbox/models/media/BaseProvider.cfc` — declares getDisplayName()
  - low `modules/contentbox/models/ui/editors/IEditor.cfc` — declares getDisplayName()
  - low `modules/contentbox/modules/contentbox-admin/modules/contentbox-ckeditor/models/CKEditor.cfc` — declares getDisplayName()
  - low `modules/contentbox/modules/contentbox-admin/modules/contentbox-markdowneditor/models/MarkdownEditor.cfc` — declares getDisplayName()
  - low `modules/contentbox/modules/contentbox-admin/modules/contentbox-security/modules/contentbox-email-twofactor/models/EmailTwoFactorProvider.cfc` — declares getDisplayName()
  - low `tests/specs/contentbox-web/contentbox/unit/ui/editors/MockEditor.cfc` — declares getDisplayName()

</details>

## Return types — a call chained on a method that declares no component

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 16 | `method 'site' has no component return type → modules/contentbox/models/system/Site.cfc` | 5 | medium | declares getsiteID(); named like the receiver 'site' (5 candidates) | `modules/contentbox/models/system/CBHelper.cfc:222` method 'site' has no component return type (chain to 'getsiteID') |
| 7 | `method 'getSite' has no component return type → modules/contentbox/models/system/Site.cfc` | 5 | medium | declares getSiteId(); named like the receiver 'Site' (5 candidates) | `modules/contentbox/widgets/Categories.cfc:45` method 'getSite' has no component return type (chain to 'getSiteId') |
| 4 | `method 'getGrammar' in qb.models.Query.QueryBuilder has no component return type` | 0 | none |  | `modules/contentbox/migrations/2022_11_23_142439_v_6_0_0_Convert-Featured-cbfs.cfc:14` method 'getGrammar' in qb.models.Query.QueryBuilder has no component return type (chain to 'convertToBooleanType') |
| 4 | `method 'getInstance' in coldbox.system.ioc.Injector has no component return type → modules/contentbox/models/system/SettingService.cfc` | 11 | low | declares importFromData() (11 candidates) | `modules/contentbox/models/system/SiteService.cfc:534` method 'getInstance' in coldbox.system.ioc.Injector has no component return type (chain to 'importFromData') |
| 4 | `method 'site' in CBHelper@contentbox has no component return type → modules/contentbox/models/system/Site.cfc` | 1 | high | declares getMediaDisk(); named like the receiver 'site' | `modules/contentbox/modules/contentbox-admin/modules/contentbox-filebrowser/handlers/Editor.cfc:12` method 'site' in CBHelper@contentbox has no component return type (chain to 'getMediaDisk') |
| 3 | `method 'getActiveContent' has no component return type → modules/contentbox/models/content/BaseContent.cfc` | 2 | low | declares getAuthorName() (2 candidates) | `modules/contentbox/models/content/BaseContent.cfc:1222` method 'getActiveContent' has no component return type (chain to 'getAuthorName') |
| 2 | `method 'getActiveContent' in Page has no component return type → modules/contentbox/models/content/ContentVersion.cfc` | 1 | medium | declares getChangelog() | `modules/contentbox/modules/contentbox-admin/views/content/quickLook.cfm:108` method 'getActiveContent' in Page has no component return type (chain to 'getChangelog') |
| 2 | `method 'getActiveContent' in contentbox.models.content.BaseContent\|contentbox.models.content.Entry\|contentbox.models.content.Page\|contentbox.models.conten…` | 8 | none |  | `modules/contentbox/modules/contentbox-admin/views/_components/editor/sidebar/InfoTable.cfm:56` method 'getActiveContent' in contentbox.models.content.BaseContent\|contentbox.models.content.Entry\|contentbox.models.content.Page\|contentbox.models.conten… |
| 2 | `method 'getCurrentEntry' has no component return type → modules/contentbox/models/comments/Comment.cfc` | 7 | low | declares hasRelatedContent(), getRelatedContent() (7 candidates) | `modules/contentbox/models/system/CBHelper.cfc:765` method 'getCurrentEntry' has no component return type (chain to 'hasRelatedContent') |
| 2 | `method 'getCurrentPage' has no component return type → modules/contentbox/models/comments/Comment.cfc` | 7 | low | declares hasRelatedContent(), getRelatedContent() (7 candidates) | `modules/contentbox/models/system/CBHelper.cfc:763` method 'getCurrentPage' has no component return type (chain to 'hasRelatedContent') |
| 2 | `method 'getCurrentPage' in contentbox.models.system.CBHelper has no component return type → modules/contentbox/models/content/BaseContent.cfc` | 5 | low | declares getSlug() (5 candidates) | `modules/contentbox/themes/default/views/_header.cfm:31` method 'getCurrentPage' in contentbox.models.system.CBHelper has no component return type (chain to 'getSlug') |
| 2 | `method 'populate' in contentStoreService@contentbox\|entryService@contentbox\|pageService@contentbox has no component return type → modules/contentbox/mode…` | 1 | medium | declares addJoinedExpiredTime() | `modules/contentbox/modules/contentbox-admin/handlers/baseContentHandler.cfc:425` method 'populate' in contentStoreService@contentbox\|entryService@contentbox\|pageService@contentbox has no component return type (chain to 'addJoinedExpired… |
| 2 | `method 'site' in CBHelper@contentBox has no component return type → modules/contentbox/models/system/Site.cfc` | 1 | high | declares getSiteRoot(); named like the receiver 'site' | `config/modules/cbfs.cfc:21` method 'site' in CBHelper@contentBox has no component return type (chain to 'getSiteRoot') |
| 2 | `method 'site' in contentbox.models.system.CBHelper has no component return type → modules/contentbox/models/BaseEntityMethods.cfc` | 1 | medium | declares getId() | `modules/contentbox/modules/contentbox-admin/views/contentTemplates/indexHelper.cfm:23` method 'site' in contentbox.models.system.CBHelper has no component return type (chain to 'getId') |
| 1 | `method 'getActiveContent' has no component return type → modules/contentbox/models/content/ContentVersion.cfc` | 4 | low | declares getIsActive() (4 candidates) | `modules/contentbox/models/content/BaseContent.cfc:1178` method 'getActiveContent' has no component return type (chain to 'getIsActive') |
| 1 | `method 'getActiveContent' in Page has no component return type → modules/contentbox/models/BaseEntityMethods.cfc` | 1 | medium | declares getDisplayCreatedDate() | `modules/contentbox/modules/contentbox-admin/views/content/pager.cfm:54` method 'getActiveContent' in Page has no component return type (chain to 'getDisplayCreatedDate') |
| 1 | `method 'getCache' has no component return type` | 4 | none |  | `modules/contentbox/modules/contentbox-admin/handlers/dashboard.cfc:275` method 'getCache' has no component return type (chain to 'clearAll') |
| 1 | `method 'getContentTemplate' in contentbox.models.content.BaseContent\|contentbox.models.content.Entry\|contentbox.models.content.Page\|contentbox.models.cont…` | 1 | high | declares getTemplateID(); named like the receiver 'ContentTemplate' | `modules/contentbox/modules/contentbox-admin/views/_components/editor/sidebar/Modifiers.cfm:33` method 'getContentTemplate' in contentbox.models.content.BaseContent\|contentbox.models.content.Entry\|contentbox.models.content.Page\|contentbox.models.cont… |
| 1 | `method 'getContentTemplate' in contentbox.models.content.BaseContent\|contentbox.models.content.Entry\|contentbox.models.content.Page\|contentbox.models.cont…` | 10 | medium | declares getMemento() | `modules/contentbox/modules/contentbox-admin/views/content/editorHelper.cfm:4` method 'getContentTemplate' in contentbox.models.content.BaseContent\|contentbox.models.content.Entry\|contentbox.models.content.Page\|contentbox.models.cont… |
| 1 | `method 'getCurrentEntry' has no component return type → modules/contentbox/models/content/BaseContent.cfc` | 1 | medium | declares getCustomFieldsAsStruct() | `modules/contentbox/models/system/CBHelper.cfc:778` method 'getCurrentEntry' has no component return type (chain to 'getCustomFieldsAsStruct') |
| 1 | `method 'getCurrentPage' has no component return type → modules/contentbox/models/content/BaseContent.cfc` | 1 | medium | declares getCustomFieldsAsStruct() | `modules/contentbox/models/system/CBHelper.cfc:776` method 'getCurrentPage' has no component return type (chain to 'getCustomFieldsAsStruct') |
| 1 | `method 'getCurrentPage' has no component return type → modules/contentbox/models/system/Site.cfc` | 5 | low | declares getSlug() (5 candidates) | `modules/contentbox/models/system/CBHelper.cfc:579` method 'getCurrentPage' has no component return type (chain to 'getSlug') |
| 1 | `method 'getInstance' in coldbox.system.ioc.Injector has no component return type → modules/contentbox/models/security/SecurityRule.cfc` | 2 | medium | declares setMessage() | `modules/contentbox/models/security/SecurityValidator.cfc:86` method 'getInstance' in coldbox.system.ioc.Injector has no component return type (chain to 'setMessage') |
| 1 | `method 'getInstance' in coldbox.system.ioc.Injector has no component return type → modules/contentbox/models/system/CBHelper.cfc` | 1 | medium | declares site() | `modules/contentbox/models/content/ContentTemplateService.cfc:142` method 'getInstance' in coldbox.system.ioc.Injector has no component return type (chain to 'site') |
| 1 | `method 'getResults' in SearchResults@contentbox has no component return type` | 1 | none |  | `tests/specs/contentbox-web/contentbox/unit/search/DBSearchTest.cfc:42` method 'getResults' in SearchResults@contentbox has no component return type (chain to 'size') |
| 1 | `method 'getWidget' has no component return type → modules/contentbox/models/ui/BaseWidget.cfc` | 26 | low | declares renderit() (25 candidates) | `modules/contentbox/models/system/CBHelper.cfc:1814` method 'getWidget' has no component return type (chain to 'renderit') |
| 1 | `method 'populate' has no component return type → modules/contentbox/models/content/BaseContent.cfc` | 6 | low | declares setSite() (6 candidates) | `modules/contentbox/modules/contentbox-admin/handlers/categories.cfc:96` method 'populate' has no component return type (chain to 'setSite') |
| 1 | `method 'populateFromStruct' in coldbox.system.core.dynamic.ObjectPopulator has no component return type → modules/contentbox/models/content/BaseContent.cfc` | 7 | low | declares setRelatedContent() (7 candidates) | `modules/contentbox/models/content/ContentService.cfc:1204` method 'populateFromStruct' in coldbox.system.core.dynamic.ObjectPopulator has no component return type (chain to 'setRelatedContent') |
| 1 | `method 'populateFromStruct' in coldbox.system.core.dynamic.ObjectPopulator has no component return type → modules/contentbox/models/system/Setting.cfc` | 6 | low | declares setSite() (6 candidates) | `modules/contentbox/models/system/SiteService.cfc:487` method 'populateFromStruct' in coldbox.system.core.dynamic.ObjectPopulator has no component return type (chain to 'setSite') |

<details><summary>Groups with several candidates</summary>

- `method 'site' has no component return type → modules/contentbox/models/system/Site.cfc` — 16 finding(s), 5 candidate(s):
  - medium `modules/contentbox/models/system/Site.cfc` — declares getsiteID(); named like the receiver 'site'
  - low `modules/contentbox/models/system/Setting.cfc` — declares getsiteID()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getsiteID()
  - low `modules/contentbox/models/content/Category.cfc` — declares getsiteID()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getsiteID()
- `method 'getSite' has no component return type → modules/contentbox/models/system/Site.cfc` — 7 finding(s), 5 candidate(s):
  - medium `modules/contentbox/models/system/Site.cfc` — declares getSiteId(); named like the receiver 'Site'
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSiteId()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSiteId()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSiteId()
  - low `modules/contentbox/models/system/Setting.cfc` — declares getSiteId()
- `method 'getInstance' in coldbox.system.ioc.Injector has no component return type → modules/contentbox/models/system/SettingService.cfc` — 4 finding(s), 11 candidate(s):
  - low `modules/contentbox/models/system/SettingService.cfc` — declares importFromData()
  - low `modules/contentbox/models/system/SiteService.cfc` — declares importFromData()
  - low `modules/contentbox/models/content/CategoryService.cfc` — declares importFromData()
  - low `modules/contentbox/models/content/ContentService.cfc` — declares importFromData()
  - low `modules/contentbox/models/content/ContentTemplateService.cfc` — declares importFromData()
  - low `modules/contentbox/models/menu/MenuService.cfc` — declares importFromData()
  - low `modules/contentbox/models/security/AuthorService.cfc` — declares importFromData()
  - low `modules/contentbox/models/security/PermissionGroupService.cfc` — declares importFromData()
  - … 3 more
- `method 'getActiveContent' has no component return type → modules/contentbox/models/content/BaseContent.cfc` — 3 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getAuthorName()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getAuthorName()
- `method 'getCurrentEntry' has no component return type → modules/contentbox/models/comments/Comment.cfc` — 2 finding(s), 7 candidate(s):
  - low `modules/contentbox/models/comments/Comment.cfc` — declares hasRelatedContent(), getRelatedContent()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares hasRelatedContent(), getRelatedContent()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares hasRelatedContent(), getRelatedContent()
  - low `modules/contentbox/models/content/CustomField.cfc` — declares hasRelatedContent(), getRelatedContent()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares hasRelatedContent(), getRelatedContent()
  - low `modules/contentbox/models/content/Stats.cfc` — declares hasRelatedContent(), getRelatedContent()
  - low `modules/contentbox/models/subscriptions/CommentSubscription.cfc` — declares hasRelatedContent(), getRelatedContent()
- `method 'getCurrentPage' has no component return type → modules/contentbox/models/comments/Comment.cfc` — 2 finding(s), 7 candidate(s):
  - low `modules/contentbox/models/comments/Comment.cfc` — declares hasRelatedContent(), getRelatedContent()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares hasRelatedContent(), getRelatedContent()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares hasRelatedContent(), getRelatedContent()
  - low `modules/contentbox/models/content/CustomField.cfc` — declares hasRelatedContent(), getRelatedContent()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares hasRelatedContent(), getRelatedContent()
  - low `modules/contentbox/models/content/Stats.cfc` — declares hasRelatedContent(), getRelatedContent()
  - low `modules/contentbox/models/subscriptions/CommentSubscription.cfc` — declares hasRelatedContent(), getRelatedContent()
- `method 'getCurrentPage' in contentbox.models.system.CBHelper has no component return type → modules/contentbox/models/content/BaseContent.cfc` — 2 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSlug()
  - low `modules/contentbox/models/system/Site.cfc` — declares getSlug()
- `method 'getActiveContent' has no component return type → modules/contentbox/models/content/ContentVersion.cfc` — 1 finding(s), 4 candidate(s):
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares getIsActive()
  - low `modules/contentbox/models/modules/Module.cfc` — declares getIsActive()
  - low `modules/contentbox/models/security/Author.cfc` — declares getIsActive()
  - low `modules/contentbox/models/system/Site.cfc` — declares getIsActive()
- `method 'getCurrentPage' has no component return type → modules/contentbox/models/system/Site.cfc` — 1 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/system/Site.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Category.cfc` — declares getSlug()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares getSlug()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares getSlug()
- `method 'getWidget' has no component return type → modules/contentbox/models/ui/BaseWidget.cfc` — 1 finding(s), 25 candidate(s):
  - low `modules/contentbox/models/ui/BaseWidget.cfc` — declares renderit()
  - low `modules/contentbox/models/ui/Paging.cfc` — declares renderit()
  - low `modules/contentbox/widgets/Archives.cfc` — declares renderit()
  - low `modules/contentbox/widgets/Categories.cfc` — declares renderit()
  - low `modules/contentbox/widgets/CommentForm.cfc` — declares renderit()
  - low `modules/contentbox/widgets/ContentStore.cfc` — declares renderit()
  - low `modules/contentbox/widgets/EntryInclude.cfc` — declares renderit()
  - low `modules/contentbox/widgets/Menu.cfc` — declares renderit()
  - … 17 more
- `method 'populate' has no component return type → modules/contentbox/models/content/BaseContent.cfc` — 1 finding(s), 6 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares setSite()
  - low `modules/contentbox/models/content/Category.cfc` — declares setSite()
  - low `modules/contentbox/models/content/ContentTemplate.cfc` — declares setSite()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares setSite()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares setSite()
  - low `modules/contentbox/models/system/Setting.cfc` — declares setSite()
- `method 'populateFromStruct' in coldbox.system.core.dynamic.ObjectPopulator has no component return type → modules/contentbox/models/content/BaseContent.cfc` — 1 finding(s), 7 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares setRelatedContent()
  - low `modules/contentbox/models/content/ContentVersion.cfc` — declares setRelatedContent()
  - low `modules/contentbox/models/content/CustomField.cfc` — declares setRelatedContent()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares setRelatedContent()
  - low `modules/contentbox/models/content/Stats.cfc` — declares setRelatedContent()
  - low `modules/contentbox/models/comments/Comment.cfc` — declares setRelatedContent()
  - low `modules/contentbox/models/subscriptions/CommentSubscription.cfc` — declares setRelatedContent()
- `method 'populateFromStruct' in coldbox.system.core.dynamic.ObjectPopulator has no component return type → modules/contentbox/models/system/Setting.cfc` — 1 finding(s), 6 candidate(s):
  - low `modules/contentbox/models/system/Setting.cfc` — declares setSite()
  - low `modules/contentbox/models/content/BaseContent.cfc` — declares setSite()
  - low `modules/contentbox/models/content/Category.cfc` — declares setSite()
  - low `modules/contentbox/models/content/ContentTemplate.cfc` — declares setSite()
  - low `modules/contentbox/models/content/Relocation.cfc` — declares setSite()
  - low `modules/contentbox/models/menu/Menu.cfc` — declares setSite()

</details>

## Method definitions — a method not found where it was looked for

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 66 | `addpermission` | 13 | low | declares addPermission (13 candidates) | `build/patches/1-0-3/Update.cfc:198` method 'addPermission' not found in cbRole |
| 16 | `getinstance` | 4 | none |  | `modules/contentbox/email_templates/author_new.cfm:2` no qualifier, not in file |
| 13 | `columns` | 2 | none |  | `build/patches/3.0.0-beta/Update.cfc:497` method 'columns' not found in dbinfo |
| 13 | `version` | 0 | none |  | `build/patches/3.0.0-beta/Update.cfc:505` method 'version' not found in dbinfo |
| 6 | `generateapitoken` | 0 | none |  | `build/patches/3.7.0/Update.cfc:194` method 'generateAPIToken' not found in Author |
| 6 | `getapitoken` | 0 | none |  | `build/patches/3.7.0/Update.cfc:193` method 'getAPIToken' not found in Author |
| 5 | `getsystemsetting` | 2 | none |  | `config/Coldbox.cfc:23` no qualifier, not in file |
| 4 | `removepermission` | 2 | low | declares removePermission (2 candidates) | `build/patches/1-0-3/Update.cfc:194` method 'removePermission' not found in cbRole |
| 4 | `setnextevent` | 0 | none |  | `build/patches/3.1.0/Update.cfc:94` method 'setNextEvent' not found in coldbox.system.web.Controller |
| 3 | `csrftoken` | 0 | none |  | `modules/contentbox/modules/contentbox-admin/modules/contentbox-security/views/security/login.cfm:24` not found in extends chain |
| 3 | `csrfverify` | 0 | none |  | `modules/contentbox/modules/contentbox-admin/modules/contentbox-security/handlers/security.cfc:55` not found in extends chain |
| 2 | `error` | 3 | none |  | `tests/resources/seeds/archive/BaseSeeder.cfc:75` no qualifier, not in file |
| 2 | `generatepasswordresettoken` | 0 | none |  | `tests/specs/contentbox-api/authSpec.cfc:224` method 'generatePasswordResetToken' not found in securityService@contentbox |
| 2 | `getcwd` | 1 | none |  | `tests/resources/seeds/archive/BaseSeeder.cfc:44` no qualifier, not in file |
| 1 | `confirm` | 2 | none |  | `tests/resources/seeds/archive/BaseSeeder.cfc:90` no qualifier, not in file |
| 1 | `delivermedia` | 3 | low | declares deliverMedia; extends BaseProvider.cfc (3 candidates) | `modules/contentbox/modules/contentbox-ui/handlers/media.cfc:87` method 'deliverMedia' not found in BaseProvider |
| 1 | `evictentity` | 0 | none |  | `modules/contentbox/models/comments/CommentService.cfc:228` not found in extends chain |
| 1 | `getcache` | 9 | low | declares getCache (3 candidates) | `modules/contentbox/modules/contentbox-admin/helpers/Mixins.cfm:33` no qualifier, not in file |
| 1 | `getdisplayexpireddate` | 0 | none |  | `modules/contentbox/modules/contentbox-admin/views/content/quickLook.cfm:15` method 'getDisplayExpiredDate' not found in Page |
| 1 | `getrecursiveslug` | 0 | none |  | `build/patches/1-0-4/Update.cfc:107` method 'getRecursiveSlug' not found in Page |
| 1 | `renderexcerpt` | 2 | low | declares renderExcerpt; beside the calling file (2 candidates) | `modules/contentbox/models/content/BaseContent.cfc:1624` not found in extends chain |
| 1 | `run` | 55 | low | declares run (49 candidates) | `tests/index.cfm:29` no qualifier, not in file |
| 1 | `validate` | 5 | low | declares validate; beside the calling file (5 candidates) | `modules/contentbox/models/exporters/BaseExporter.cfc:60` no qualifier, not in file |
| 1 | `view` | 2 | none |  | `modules/contentbox/modules/contentbox-admin/helpers/Mixins.cfm:11` no qualifier, not in file |

<details><summary>Groups with several candidates</summary>

- `addpermission` — 66 finding(s), 13 candidate(s):
  - low `build/patches/3.0.0-beta/Update.cfc:240` — declares addPermission
  - low `build/patches/3.0.0-rc/Update.cfc:241` — declares addPermission
  - low `build/patches/3.0.0/Update.cfc:244` — declares addPermission
  - low `build/patches/3.1.0/Update.cfc:186` — declares addPermission
  - low `build/patches/3.5.0/Update.cfc:260` — declares addPermission
  - low `build/patches/3.5.1/Update.cfc:260` — declares addPermission
  - low `build/patches/3.6.0/Update.cfc:260` — declares addPermission
  - low `build/patches/3.7.0/Update.cfc:302` — declares addPermission
  - … 5 more
- `removepermission` — 4 finding(s), 2 candidate(s):
  - low `modules/contentbox/migrations/util/MigrationUtils.cfm:178` — declares removePermission
  - low `modules/contentbox/modules/contentbox-admin/handlers/authors.cfc:747` — declares removePermission
- `delivermedia` — 1 finding(s), 3 candidate(s):
  - low `modules/contentbox/models/media/CFContentMediaProvider.cfc:26` — declares deliverMedia; extends BaseProvider.cfc
  - low `modules/contentbox/models/media/ForwardMediaProvider.cfc:26` — declares deliverMedia; extends BaseProvider.cfc
  - low `modules/contentbox/models/media/RelocationMediaProvider.cfc:26` — declares deliverMedia; extends BaseProvider.cfc
- `getcache` — 1 finding(s), 3 candidate(s):
  - low `modules/contentbox/models/content/BaseContent.cfc:212` — declares getCache
  - low `modules/contentbox/models/security/SecurityService.cfc:30` — declares getCache
  - low `modules/contentbox/modules/contentbox-admin/modules/contentbox-security/modules/contentbox-email-twofactor/models/EmailTwoFactorProvider.cfc:18` — declares getCache
- `renderexcerpt` — 1 finding(s), 2 candidate(s):
  - low `modules/contentbox/models/content/Entry.cfc:81` — declares renderExcerpt; beside the calling file
  - low `modules/contentbox/models/content/Page.cfc:118` — declares renderExcerpt; beside the calling file
- `run` — 1 finding(s), 49 candidate(s):
  - low `tests/resources/SeedData.cfc:26` — declares run
  - low `build/BuildDocs.cfc:43` — declares run
  - low `tests/resources/seeds/TestFixtures.cfc:9` — declares run
  - low `tests/specs/contentbox-api/AuthorsSpec.cfc:30` — declares run
  - low `tests/specs/contentbox-api/CategoriesSpec.cfc:26` — declares run
  - low `tests/specs/contentbox-api/ContentStoreSpec.cfc:28` — declares run
  - low `tests/specs/contentbox-api/EchoSpec.cfc:19` — declares run
  - low `tests/specs/contentbox-api/EntriesSpec.cfc:28` — declares run
  - … 41 more
- `validate` — 1 finding(s), 5 candidate(s):
  - low `modules/contentbox/models/exporters/DataExporter.cfc:47` — declares validate; beside the calling file
  - low `modules/contentbox/models/exporters/FileExporter.cfc:56` — declares validate; beside the calling file
  - low `modules/contentbox/models/exporters/ICBExporter.cfc:13` — declares validate; beside the calling file
  - low `modules/contentbox/models/ui/Widget.cfc:37` — declares validate
  - low `modules/contentbox/models/validators/UniqueSiteFieldValidator.cfc:32` — declares validate

</details>

## Object references — a component path that names no file

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 27 | `component 'coldbox.system.orm.hibernate.util.ORMUtilFactory' does not exist (calling 'getORMUtil')` | 0 | none |  | `build/patches/1-0-7/Update.cfc:156` component 'coldbox.system.orm.hibernate.util.ORMUtilFactory' does not exist (calling 'getORMUtil') |
| 27 | `component 'coldbox.system.orm.hibernate.util.ORMUtilFactory' does not exist (chain hop 'getORMUtil' to 'getDefaultDatasource')` | 0 | none |  | `build/patches/1-0-7/Update.cfc:156` component 'coldbox.system.orm.hibernate.util.ORMUtilFactory' does not exist (chain hop 'getORMUtil' to 'getDefaultDatasource') |
| 13 | `component 'cborm.models.util.ORMUtilFactory' does not exist (calling 'getORMUtil')` | 0 | none |  | `build/patches/3.0.0-beta/Update.cfc:511` component 'cborm.models.util.ORMUtilFactory' does not exist (calling 'getORMUtil') |
| 13 | `component 'cborm.models.util.ORMUtilFactory' does not exist (chain hop 'getORMUtil' to 'getDefaultDatasource')` | 0 | none |  | `build/patches/3.0.0-beta/Update.cfc:511` component 'cborm.models.util.ORMUtilFactory' does not exist (chain hop 'getORMUtil' to 'getDefaultDatasource') |
| 4 | `extends baseHandler, whose chain breaks at cborm.models.resources.BaseHandler, which does not resolve; 2 inherited calls not checked` | ? | none |  | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/authors.cfc:5` extends baseHandler, whose chain breaks at cborm.models.resources.BaseHandler, which does not resolve; 2 inherited calls not checked |
| 3 | `extends baseContentHandler, whose chain breaks at cborm.models.resources.BaseHandler, which does not resolve; 2 inherited calls not checked` | ? | none |  | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/contentStore.cfc:6` extends baseContentHandler, whose chain breaks at cborm.models.resources.BaseHandler, which does not resolve; 2 inherited calls not checked |
| 3 | `extends baseHandler, whose chain breaks at cborm.models.resources.BaseHandler, which does not resolve; 3 inherited calls not checked` | ? | none |  | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/comments.cfc:7` extends baseHandler, whose chain breaks at cborm.models.resources.BaseHandler, which does not resolve; 3 inherited calls not checked |
| 1 | `base component does not resolve; 1 inherited call not checked` | ? | none |  | `modules/contentbox/models/security/SecurityValidator.cfc:9` base component does not resolve; 1 inherited call not checked |
| 1 | `base component does not resolve; 15 inherited calls not checked` | ? | none |  | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/baseHandler.cfc:37` base component does not resolve; 15 inherited calls not checked |
| 1 | `chained on 'cbfs', which is not found (calling 'getDisks')` | 0 | none |  | `modules/contentbox/modules/contentbox-admin/handlers/sites.cfc:67` chained on 'cbfs', which is not found (calling 'getDisks') |
| 1 | `chained on 'getCache', which is not found (calling 'getOrSet')` | 1 | none |  | `modules/contentbox/modules/contentbox-admin/helpers/Mixins.cfm:33` chained on 'getCache', which is not found (calling 'getOrSet') |
| 1 | `chained on 'jwtAuth', which is not found (calling 'fromUser')` | 0 | none |  | `modules/contentbox/modules/contentbox-admin/handlers/baseContentHandler.cfc:331` chained on 'jwtAuth', which is not found (calling 'fromUser') |
| 1 | `extends baseHandler, whose chain breaks at cborm.models.resources.BaseHandler, which does not resolve; 13 inherited calls not checked` | ? | none |  | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/auth.cfc:4` extends baseHandler, whose chain breaks at cborm.models.resources.BaseHandler, which does not resolve; 13 inherited calls not checked |
| 1 | `extends baseHandler, whose chain breaks at cborm.models.resources.BaseHandler, which does not resolve; 4 inherited calls not checked` | ? | none |  | `modules/contentbox/modules/contentbox-api/modules/contentbox-api-v1/handlers/baseContentHandler.cfc:4` extends baseHandler, whose chain breaks at cborm.models.resources.BaseHandler, which does not resolve; 4 inherited calls not checked |
