# masa-c — unresolved findings and candidates

Every finding below is still reported. A candidate is a liberal match offered for a person to weigh, never an answer the resolver took: **high** is the only component declaring every method the function calls on the receiver (or the only one, named like it); **medium** is a sole match on weaker evidence, or the one named like the receiver among several; **low** is one of several. *Defined* is how many indexed files declare a method of that name: 0 means it is missing from the workspace, more means the resolver could not connect the call to it.

| Category | Findings | high | medium | low | none | method defined nowhere |
|---|---:|---:|---:|---:|---:|---:|
| variable | 4561 | 1833 | 851 | 767 | 1110 | 88 |
| return-type | 637 | 125 | 269 | 235 | 8 | 2 |
| method | 103 | 0 | 30 | 58 | 15 | 15 |
| object | 100 | 0 | 0 | 0 | 100 | 12 |

## Variable definitions — a receiver whose component is unknown

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 304 | `arguments.event → core/mura/event.cfc` | 13 | medium | declares getValue(); named like the receiver 'event' (13 candidates) | `core/modules/v1/deprecation/model/handlers/handler.cfc:20` variable 'arguments.event' has no component ref |
| 183 | `event → core/mura/event.cfc` | 7 | medium | declares getContentRenderer(); named like the receiver 'event' (7 candidates) | `core/modules/v1/comments/index.cfm:188` variable 'event' has no component ref |
| 146 | `rc.sitebean` | 1 | none |  | `admin/core/views/csettings/editsite.cfm:75` variable 'rc.siteBean' has no component ref |
| 144 | `mmrbf → core/mura/resourceBundle/resourceBundle.cfc` | 2 | low | declares getKeyValue() (2 candidates) | `admin/core/utilities/formbuilder/templates/dataset-create.cfm:79` variable 'mmRBF' has no component ref |
| 139 | `arguments.renderer → core/mura/content/contentRenderer.cfc` | 2 | low | declares renderIcon() (2 candidates) | `core/mura/content/contentRendererUtility.cfc:48` variable 'arguments.renderer' has no component ref |
| 128 | `arguments.item → core/mura/content/contentBean.cfc` | 13 | high | declares getValue(), getDisplayIntervalDesc(), getMenuTitle(), getTarget(), getURL(), getImageURL() | `core/modules/v1/collection/includes/dsp_content_list.cfm:156` variable 'arguments.item' has no component ref |
| 120 | `rc.contentbean → core/mura/content/contentBean.cfc` | 1 | high | declares getStats(), getType(), getFileID(), getIsLocked(), getRemoteURL(), getURL(); named like the receiver 'contentBean' | `admin/core/views/carch/audit.cfm:84` variable 'rc.contentBean' has no component ref |
| 106 | `arguments.feedbean` | 4 | none |  | `core/mura/content/feed/feedGateway.cfc:145` variable 'arguments.feedBean' has no component ref |
| 100 | `rc.contentbean` | 3 | none |  | `admin/core/controllers/carch.cfc:408` variable 'rc.contentBean' has no component ref |
| 95 | `content → core/mura/content/contentBean.cfc` | 4 | high | declares getContentID(), getURL(); named like the receiver 'content' | `admin/core/views/carch/approvalaction.cfm:12` variable 'content' has no component ref |
| 80 | `arguments.bean` | 9 | none |  | `core/mura/client/api/json/v1/jsonApiUtility.cfc:1748` variable 'arguments.bean' has no component ref |
| 80 | `attributebean → core/mura/extend/extendAttribute.cfc` | 2 | high | declares getName(), getAdminOnly(), getType(), setType(), getHint(), getLabel(), renderAttribute(), getValidation() | `admin/core/views/carch/loadextendedattributes.cfm:119` variable 'attributeBean' has no component ref |
| 76 | `variables.filewriter → core/mura/fileWriter.cfc` | 1 | high | declares writeFile(), createDir(), moveFile(); named like the receiver 'fileWriter' | `core/mura/autoUpdater/autoUpdater.cfc:123` variable 'variables.fileWriter' has no component ref |
| 72 | `entity` | 1 | none |  | `core/mura/Handler/standardEventsHandler.cfc:884` variable 'entity' has no component ref |
| 72 | `variables.configbean → core/mura/configBean.cfc` | 2 | medium | declares getContext(), getServerPort(); named like the receiver 'configBean' (2 candidates) | `core/modules/v1/login/model/providers/baseLoginProvider.cfc:12` variable 'variables.configBean' has no component ref |
| 70 | `subtype → core/mura/extend/extendSubType.cfc` | 1 | high | declares getHasSummary(), getHasBody() | `admin/core/views/carch/edit.cfm:262` variable 'subType' has no component ref |
| 51 | `application.classextensionmanager → core/mura/extend/extendManager.cfc` | 1 | medium | declares getSubTypes() | `admin/common/layouts/includes/nav.cfm:75` variable 'application.classExtensionManager' has no component ref |
| 50 | `rc.item` | 1 | none |  | `admin/core/views/carch/hist.cfm:146` variable 'rc.item' has no component ref |
| 45 | `item → core/mura/content/contentBean.cfc` | 1 | high | declares getMenuTitle(), getURL() | `core/modules/v1/calendar/configurator.cfm:130` variable 'item' has no component ref |
| 43 | `stats → core/mura/content/contentStatsBean.cfc` | 1 | high | declares getLockID(), getLockType() | `admin/assets/js/frontendtools.js.cfm:709` variable 'stats' has no component ref |
| 42 | `variables.feedbean → core/mura/content/feed/feedBean.cfc` | 14 | high | declares getType(), getDisplayList(), setDisplayList(), getIsActive(), getIsNew(), renderName(), getIterator(), getDisplayComments(), getDisplayRatings(), ge… | `core/modules/v1/feed/index.cfm:95` variable 'variables.feedBean' has no component ref |
| 41 | `rc.item → core/mura/content/contentBean.cfc` | 1 | high | declares getactive(), getapproved(), getapprovalstatus(), getchangesetID(), getContentHistID(), getContentID(), getSiteID(), gettype(), getmenutitle(), getma… | `admin/core/views/carch/audit.cfm:147` variable 'rc.item' has no component ref |
| 40 | `request.contentbean` | 1 | none |  | `admin/core/utilities/modal/toolbar.cfm:173` variable 'request.contentBean' has no component ref |
| 37 | `arguments.bundle → core/mura/cfobject.cfc` | 13 | low | declares getValue() (13 candidates) | `core/mura/publisher.cfc:679` variable 'arguments.Bundle' has no component ref |
| 37 | `local.item → core/mura/cfobject.cfc` | 13 | low | declares getValue() (13 candidates) | `admin/core/views/cusers/inc/dsp_users_list.cfm:112` variable 'local.item' has no component ref |
| 36 | `extendsetbean → core/mura/extend/extendSet.cfc` | 1 | high | declares getStyle(), getCategoryID(), getExtendSetID(), getName(), getAttributes(); named like the receiver 'extendSetBean' | `admin/core/views/carch/loadextendedattributes.cfm:109` variable 'extendSetBean' has no component ref |
| 36 | `feed` | 2 | none |  | `admin/core/views/carch/objectclass/legacy/dsp_related_content_configurator.cfm:87` variable 'feed' has no component ref |
| 36 | `rcsbean → core/mura/extend/extendRelatedContentSetBean.cfc` | 21 | high | declares setName(), setRelatedContentSetId(), getName(), getRelatedContentSetID(), getEntityType(), exists(), getRelatedContentQuery(), getAvailableSubTypes(… | `admin/core/views/carch/loadselectedrelatedcontent.cfm:17` variable 'rcsBean' has no component ref |
| 35 | `arguments.contentbean → core/mura/content/contentBean.cfc` | 26 | high | declares getSiteID(), getIsNew(), getContentID(), getRemoteID(), getFilename(), getTitle(), getURLTitle(), getVersionHistoryIterator(); named like the receiv… | `core/mura/content/contentManager.cfc:2922` variable 'arguments.contentBean' has no component ref |
| 34 | `contentbean → core/mura/content/contentBean.cfc` | 6 | high | declares getFilename(), getType(); named like the receiver 'contentBean' | `core/modules/v1/favorites/ajax/saveFavorite.cfm:86` variable 'contentBean' has no component ref |
| 34 | `rc.bean` | 45 | none |  | `admin/core/controllers/cwebservice.cfc:109` variable 'rc.bean' has no component ref |
| 32 | `arguments.addressbean` | 1 | none |  | `core/mura/user/userDAO.cfc:754` variable 'arguments.addressBean' has no component ref |
| 32 | `attributes.bean` | 4 | none |  | `core/mura/customtags/filetools.cfm:9` variable 'attributes.bean' has no component ref |
| 32 | `entity → core/mura/bean/bean.cfc` | 2 | low | declares getDynamic(), getTable(), getEntityName() (2 candidates) | `core/mura/bean/beanFactory.cfc:185` variable 'entity' has no component ref |
| 31 | `currentbean` | 6 | none |  | `admin/core/views/carch/dsp_close_compact_display.cfm:99` variable 'currentBean' has no component ref |
| 30 | `bean → core/mura/content/contentBean.cfc` | 25 | high | declares getAllValues(), getIsNew(), getContentID(), getSubType() | `core/modules/v1/component/index.cfm:83` variable 'bean' has no component ref |
| 28 | `attributes.attributebean → core/mura/extend/extendAttribute.cfc` | 1 | high | declares getAttributeID(), getName(), getLabel(), getType(), getDefaultvalue(), getHint(), getRequired(), getValidation(), getRegex(), getMessage(), getOptio… | `admin/core/views/cextend/dsp_attribute_form.cfm:108` variable 'attributes.attributeBean' has no component ref |
| 28 | `item` | 1 | none |  | `admin/core/views/cchain/edit.cfm:63` variable 'item' has no component ref |
| 28 | `parentbean → core/mura/content/contentBean.cfc` | 1 | high | declares getContentHistID(), getURL(), getActive(), getIsOnDisplay() | `admin/core/views/carch/dsp_close_compact_display.cfm:108` variable 'parentBean' has no component ref |
| 28 | `variables.contentgateway → core/mura/content/contentGatewayAdobe.cfc` | 2 | high | declares getNest(), getTop() | `core/mura/content/contentManager.cfc:165` variable 'variables.contentGateway' has no component ref |
| 26 | `apiutility → core/mura/client/api/json/v1/jsonApiUtility.cfc` | 1 | high | declares getApiVersion(), getParamsWithOutMethod(), getSerializer() | `core/mura/Handler/standardEventsHandler.cfc:813` variable 'apiUtility' has no component ref |
| 26 | `arguments.content → core/mura/content/contentBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/content/contentRendererUtility.cfc:1919` variable 'arguments.content' has no component ref |
| 26 | `local.formbean → core/mura/content/contentBean.cfc` | 6 | high | declares getSubType(), getBody(), getContentID(), getResponseChart(), getResponseMessage(), getDisplayTitle() | `core/modules/v1/form/index.cfm:77` variable 'local.formBean' has no component ref |
| 26 | `request.userbean → core/mura/user/userBean.cfc` | 6 | medium | declares getCategoryID(); named like the receiver 'userBean' (6 candidates) | `core/modules/v1/editprofile/dsp_categories_next.cfm:87` variable 'request.userBean' has no component ref |
| 25 | `arguments.contentbean → core/mura/content/feed/feedBean.cfc` | 4 | high | declares getContentID(), getParentID(), getsiteid(), gettype() | `core/mura/content/contentDAO.cfc:1167` variable 'arguments.contentBean' has no component ref |
| 25 | `arguments.deleted → core/mura/content/contentBean.cfc` | 9 | high | declares getEntityName(), getValue(), setValue(), getDisplayRegion(), getRelatedContentQuery(), getContentHistID(), getAllValues(), valueExists(), getType(),… | `core/mura/trash/trashManager.cfc:176` variable 'arguments.deleted' has no component ref |
| 25 | `feed → core/mura/content/feed/feedBean.cfc` | 2 | medium | declares getImageSize(), getImageWidth(), getImageHeight(), getdisplaylist(), getAvailabledisplaylist(); named like the receiver 'feed' (2 candidates) | `core/modules/v1/calendar/configurator.cfm:258` variable 'feed' has no component ref |
| 23 | `arguments.event → core/mura/content/contentBean.cfc` | 4 | low | declares getcontentID() (4 candidates) | `core/mura/Handler/standardEventsHandler.cfc:126` variable 'arguments.event' has no component ref |
| 23 | `arguments.sourceiterator → core/mura/iterator/queryIterator.cfc` | 7 | high | declares getNextN(), getRecordCount(), getPageIndex(), getPageQuery(), getPageIDList(), setPageQuery() | `core/mura/content/contentDAO.cfc:116` variable 'arguments.sourceIterator' has no component ref |
| 23 | `chain` | 4 | none |  | `admin/core/views/cchain/edit.cfm:24` variable 'chain' has no component ref |
| 22 | `arguments.bundle → core/mura/settings/settingsBundle.cfc` | 13 | low | declares getValue(), unpackFiles(), renameFiles(), cleanUp() (2 candidates) | `core/mura/publisher.cfc:136` variable 'arguments.Bundle' has no component ref |
| 21 | `arguments.contentbean` | 14 | none |  | `core/mura/content/contentUtility.cfc:392` variable 'arguments.contentBean' has no component ref |
| 21 | `arguments.feed → core/mura/content/feed/feedBean.cfc` | 9 | high | declares getEntityName(), setSortBy(), setOrderBy(), setIncludeHomePage(), setShowNavOnly(), setUseCategoryIntersect(), setShowExcludeSearch(), setValue(), s… | `core/mura/client/api/json/v1/jsonApiUtility.cfc:2989` variable 'arguments.feed' has no component ref |
| 21 | `relatedcontentsets[] → core/mura/extend/extendRelatedContentSetBean.cfc` | 1 | high | declares getRelatedContentSetId(), getEntityType(), getDisplayName() | `admin/core/views/carch/loadrelatedcontent.cfm:101` variable 'relatedContentSets[]' has no component ref |
| 20 | `arguments.entity` | 9 | none |  | `core/mura/client/api/json/v1/jsonApiUtility.cfc:2101` variable 'arguments.entity' has no component ref |
| 19 | `rc.changeset → core/mura/content/changeset/changesetBean.cfc` | 1 | high | declares getPublished(), getIsNew(), getChangesetID(); named like the receiver 'changeset' | `admin/core/views/cchangesets/dsp_secondary_menu.cfm:84` variable 'rc.changeset' has no component ref |
| 18 | `arguments.event → core/mura/plugin/pluginStandardEventWrapper.cfc` | 1 | medium | declares handle() | `core/mura/Handler/standardEventsHandler.cfc:246` variable 'arguments.event' has no component ref |
| 18 | `event → core/mura/content/contentBean.cfc` | 7 | medium | declares getNextN(); named like the receiver 'ContentBean' (7 candidates) | `core/modules/v1/gallery/index.cfm:116` variable 'event' has no component ref |
| 18 | `request.servletevent → core/mura/servletEvent.cfc` | 13 | medium | declares getValue(); named like the receiver 'servletEvent' (13 candidates) | `core/modules/v1/nav/dsp_tag_line.cfm:77` variable 'request.servletEvent' has no component ref |
| 17 | `arguments.event → core/mura/MasaScope.cfc` | 13 | low | declares getValue(), setValue(), getContentBean() (3 candidates) | `core/mura/Handler/standardEventsHandler.cfc:463` variable 'arguments.event' has no component ref |
| 17 | `arguments.iterator → core/mura/iterator/queryIterator.cfc` | 1 | high | declares next(), hasPrevious(), hasNext(), getRecordIndex() | `core/modules/v1/collection/includes/dsp_content_list.cfm:145` variable 'arguments.iterator' has no component ref |
| 17 | `node` | 15 | none |  | `admin/assets/js/frontendtools.js.cfm:673` variable 'node' has no component ref |
| 16 | `arguments.renderer` | 3 | none |  | `core/mura/content/contentRendererUtility.cfc:84` variable 'arguments.renderer' has no component ref |
| 16 | `iterator → core/mura/iterator/queryIterator.cfc` | 1 | medium | declares setPage() | `core/modules/v1/collection/includes/dsp_pagination.cfm:100` variable 'iterator' has no component ref |
| 15 | `approvalrequest` | 3 | none |  | `admin/core/views/carch/approvalaction.cfm:5` variable 'approvalRequest' has no component ref |
| 15 | `it → core/mura/iterator/queryIterator.cfc` | 1 | medium | declares next() | `admin/core/views/cchain/edit.cfm:61` variable 'it' has no component ref |
| 15 | `pluginconfig → core/mura/plugin/pluginConfig.cfc` | 1 | high | declares getDirectory(), setSetting(); named like the receiver 'pluginConfig' | `core/mura/plugin/pluginExecutor.cfc:141` variable 'pluginConfig' has no component ref |
| 14 | `arguments.event` | 2 | none |  | `core/mura/Handler/standardEventsHandler.cfc:214` variable 'arguments.event' has no component ref |
| 14 | `arguments.renderer → core/mura/MasaScope.cfc` | 1 | medium | declares event() | `core/mura/content/contentRendererUtility.cfc:91` variable 'arguments.renderer' has no component ref |
| 14 | `attributes.bean → core/mura/content/contentBean.cfc` | 14 | high | declares getType(), getSiteID(), getValue(), getcontentid(), getContentHistID(), getFileID() | `core/mura/customtags/fileselector.cfm:17` variable 'attributes.bean' has no component ref |
| 14 | `obj → core/mura/user/addressBean.cfc` | 26 | low | declares setSiteID(), getPrimaryKey(), loadBy() (5 candidates) | `core/mura/bean/bean.cfc:364` variable 'obj' has no component ref |
| 13 | `attributes.contentbean` | 13 | none |  | `admin/core/views/carch/form/dsp_categories_nest.cfm:99` variable 'attributes.contentBean' has no component ref |
| 13 | `rc.changesets → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next(), pageCount(), getFirstRecordOnPageIndex(), getLastRecordOnPageIndex(), getRecordcount(), getPageIndex() | `admin/core/views/cchangesets/list.cfm:176` variable 'rc.changesets' has no component ref |
| 13 | `sourceattribute → core/mura/extend/extendAttribute.cfc` | 2 | high | declares getName(), getHint(), getType(), getOrderNo(), getIsActive(), getAdminOnly(), getRequired(), getValidation(), getRegex(), getMessage(), getLabel(), … | `core/mura/extend/extendManager.cfc:1645` variable 'sourceAttribute' has no component ref |
| 13 | `user → core/mura/user/userBean.cfc` | 15 | high | declares getIsNew(), getFullName(); named like the receiver 'user' | `admin/core/views/carch/statusmodal.cfm:87` variable 'user' has no component ref |
| 13 | `variables.configbean` | 0 | none |  | `core/modules/v1/login/model/providers/azureADLoginProvider.cfc:7` variable 'variables.configBean' has no component ref |
| 13 | `variables.formbean → core/mura/cfobject.cfc` | 13 | low | declares getValue() (13 candidates) | `core/modules/v1/dataresponses/dsp_detail.cfm:88` variable 'variables.formBean' has no component ref |
| 13 | `variables.instance.sourceiterator → core/mura/iterator/queryIterator.cfc` | 7 | high | declares getNextN(), getRecordCount(), getPageIndex(), getPageQuery(), getRecordIdField(), getPageIDList(), setPageQuery() | `core/mura/extend/extendData.cfc:235` variable 'variables.instance.sourceIterator' has no component ref |
| 12 | `arguments.bean → core/mura/bean/bean.cfc` | 9 | high | declares getEntityName(), allowAccess(); named like the receiver 'bean' | `core/mura/client/api/json/v1/jsonApiUtility.cfc:1694` variable 'arguments.bean' has no component ref |
| 12 | `arguments.bean → core/mura/content/contentBean.cfc` | 3 | high | declares getFileExt(), getType(), getValue() | `core/mura/content/contentManager.cfc:2770` variable 'arguments.bean' has no component ref |
| 12 | `arguments.rc.contentbean` | 1 | none |  | `admin/core/controllers/carch.cfc:383` variable 'arguments.rc.contentBean' has no component ref |
| 12 | `chains → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next() | `admin/core/views/cchain/list.cfm:89` variable 'chains' has no component ref |
| 12 | `responseobject` | 3 | none |  | `core/mura/client/api/feed/v1/feedApiUtility.cfc:107` variable 'responseObject' has no component ref |
| 11 | `exampleentity` | 14 | none |  | `core/mura/client/api/json/v1/jsonApiUtility.cfc:1552` variable 'exampleEntity' has no component ref |
| 11 | `filemetadata` | 15 | none |  | `core/mura/customtags/filetools.cfm:21` variable 'fileMetaData' has no component ref |
| 11 | `obj → core/mura/bean/bean.cfc` | 1 | medium | declares setAddedObjectValues() | `core/mura/bean/bean.cfc:204` variable 'obj' has no component ref |
| 11 | `reg → core/mura/bean/beanEntity.cfc` | 2 | high | declares getScaffold(), getDynamic(), getIsNew(), getCode(), save() | `core/mura/bean/bean.cfc:1458` variable 'reg' has no component ref |
| 11 | `renderer → core/mura/content/contentRenderer.cfc` | 1 | medium | declares showItemMeta() | `core/mura/Handler/standardEventsHandler.cfc:420` variable 'renderer' has no component ref |
| 10 | `arguments.entity → core/mura/content/contentBean.cfc` | 3 | high | declares hasImage(), getImageURL(), getSiteID() | `core/mura/client/api/json/v1/jsonApiUtility.cfc:3590` variable 'arguments.entity' has no component ref |
| 10 | `bean` | 10 | none |  | `core/mura/content/contentDAO.cfc:1699` variable 'bean' has no component ref |
| 10 | `contentbean` | 15 | none |  | `core/mura/content/contentUtility.cfc:1613` variable 'contentBean' has no component ref |
| 10 | `filemetadata → core/mura/content/contentFileMetaDataBean.cfc` | 1 | high | declares hasImageFileExt(), getUrlForImage(), getCaption(), getCredits(), getAltText(), getRemoteID(), getRemoteURL(), getRemotePubDate(), getRemoteSource(),… | `admin/core/views/carch/loadfilemetadata.cfm:99` variable 'fileMetaData' has no component ref |
| 10 | `local.parentbean → core/mura/content/contentBean.cfc` | 15 | high | declares getIsNew(), setTitle(), getContentID(), getSiteID() | `core/mura/content/contentDAO.cfc:1293` variable 'local.parentBean' has no component ref |
| 10 | `rc.pluginconfig → core/mura/plugin/pluginConfig.cfc` | 3 | high | declares getName(), getCategory(), getVersion(), getProvider(), getProviderURL(), getPluginID(), getPackage(), getModuleID(); named like the receiver 'plugin… | `admin/core/views/csettings/updatepluginversion.cfm:88` variable 'rc.pluginConfig' has no component ref |
| 10 | `service → core/mura/client/api/oauth/oauthClientBean.cfc` | 1 | high | declares getClientID(), getName(), getGrantType(), getLastUpdate() | `admin/core/views/cwebservice/list.cfm:118` variable 'service' has no component ref |
| 9 | `action → core/mura/content/approval/approvalActionBean.cfc` | 1 | high | declares getActionType(), getComments(), getCreated(), getUser() | `admin/core/views/carch/statusmodal.cfm:190` variable 'action' has no component ref |
| 9 | `arguments.object → core/mura/bean/bean.cfc` | 2 | low | declares getValidations() (2 candidates) | `core/mura/bean/beanValidator.cfc:82` variable 'arguments.object' has no component ref |
| 8 | `application.classextensionmanager → core/mura/extend/extendSubType.cfc` | 1 | medium | declares getExtendSetBean() | `admin/core/controllers/cextend.cfc:162` variable 'application.classExtensionManager' has no component ref |
| 8 | `local.stats → core/mura/content/contentStatsBean.cfc` | 1 | high | declares getLockID(), setLockID() | `admin/core/controllers/carch.cfc:599` variable 'local.stats' has no component ref |
| 8 | `stats → core/mura/content/contentBean.cfc` | 2 | low | declares getMajorVersion(), getMinorVersion(), setMajorVersion(), setMinorVersion(), save() (2 candidates) | `core/mura/content/contentUtility.cfc:1504` variable 'stats' has no component ref |
| 7 | `archivebean → core/mura/content/contentBean.cfc` | 15 | low | declares getIsNew(), getURL(), getFilename() (2 candidates) | `core/mura/Handler/standardEventsHandler.cfc:475` variable 'archiveBean' has no component ref |
| 7 | `arguments.data.bean → core/mura/bean/bean.cfc` | 11 | medium | declares getFeed(); named like the receiver 'bean' (11 candidates) | `core/tests/specs/mura/core/entities.cfc:378` variable 'arguments.data.bean' has no component ref |
| 7 | `arguments.event → core/mura/content/contentRenderer.cfc` | 2 | low | declares getCurrentURL() (2 candidates) | `core/mura/Handler/standardEventsHandler.cfc:310` variable 'arguments.event' has no component ref |
| 7 | `arguments.feedbean → core/mura/content/feed/feedBean.cfc` | 4 | medium | declares getRestrictGroups(), getSiteID(), getIsNew(), getRestricted(); named like the receiver 'feedBean' (2 candidates) | `core/mura/content/feed/feedManager.cfc:477` variable 'arguments.feedBean' has no component ref |
| 7 | `bundle → core/mura/cfobject.cfc` | 13 | low | declares getValue() (13 candidates) | `core/mura/publisher.cfc:268` variable 'Bundle' has no component ref |
| 7 | `comment → core/mura/content/contentCommentBean.cfc` | 15 | high | declares getIsNew(), getUserID(), getEmail(), getName(), setUserID() | `core/mura/Handler/standardEventsHandler.cfc:164` variable 'comment' has no component ref |
| 7 | `commenter → core/mura/content/contentCommenterBean.cfc` | 15 | high | declares loadBy(), setRemoteID(), setName(), setEmail(), save(), getCommenterID() | `core/mura/Handler/standardEventsHandler.cfc:166` variable 'commenter' has no component ref |
| 7 | `content` | 1 | none |  | `admin/core/views/cdashboard/loadrecentcomments.cfm:100` variable 'content' has no component ref |
| 7 | `formdatabean → core/mura/bean/bean.cfc` | 13 | low | declares getValue(), getErrors() (4 candidates) | `core/modules/v1/datacollection/dsp_response.cfm:99` variable 'formDataBean' has no component ref |
| 7 | `qs` | 0 | none |  | `core/mura/bean/beanORM.cfc:963` variable 'qs' has no component ref |
| 7 | `rc.it → core/mura/iterator/queryIterator.cfc` | 1 | medium | declares getRecordcount() | `admin/core/views/cusers/advancedsearch.cfm:266` variable 'rc.it' has no component ref |
| 7 | `rc.razunasettings → core/mura/content/file/razuna/razunaSettingsBean.cfc` | 1 | high | declares getApiKey(), getHostID(), getHostName(); named like the receiver 'razunaSettings' | `admin/core/controllers/razuna.cfc:41` variable 'rc.razunaSettings' has no component ref |
| 7 | `section → core/mura/content/contentBean.cfc` | 4 | high | declares exists(), gettype(), getKidsCategoryQuery(), getFilename(), getContentID() | `core/modules/v1/category_summary/index.cfm:91` variable 'section' has no component ref |
| 6 | `arguments.contentbean → core/mura/bean/beanExtendable.cfc` | 26 | low | declares getSiteid(), getSubType(), getType() (6 candidates) | `core/mura/content/contentManager.cfc:2231` variable 'arguments.contentBean' has no component ref |
| 6 | `arguments.deleted` | 7 | none |  | `core/mura/trash/trashManager.cfc:281` variable 'arguments.deleted' has no component ref |
| 6 | `arguments.event → core/mura/settings/settingsBean.cfc` | 1 | medium | declares getThemeIncludePath() | `core/mura/Handler/standardEventsHandler.cfc:337` variable 'arguments.event' has no component ref |
| 6 | `arguments.restored → core/mura/bean/bean.cfc` | 9 | high | declares getEntityName(), valueExists(), getAllValues(), getPrimaryKey(), getvalue() | `core/mura/trash/trashManager.cfc:410` variable 'arguments.restored' has no component ref |
| 6 | `calendar → core/mura/content/contentBean.cfc` | 4 | high | declares getContentID(), getMenuTitle() | `core/modules/v1/calendar/index.cfm:155` variable 'calendar' has no component ref |
| 6 | `event → core/mura/content/contentNavBean.cfc` | 3 | low | declares getContentBean(), setValue(), getValue() (3 candidates) | `core/mura/content/contentRendererUtility.cfc:337` variable 'event' has no component ref |
| 6 | `event → core/mura/settings/settingsBean.cfc` | 1 | high | declares getExtranetPublicRegNotify(), getMailServerIP(), getMailserverUsername(), getMailserverPassword(), getMailserverUsernameEmail() | `core/mura/utility.cfc:557` variable 'event' has no component ref |
| 6 | `publishedversion → core/mura/content/contentBean.cfc` | 1 | high | declares getApproved(), getMenuTitle(), getLastUpdate(), getLastUpdateBy() | `admin/core/views/carch/draftpromptdata.cfm:118` variable 'publishedVersion' has no component ref |
| 6 | `rbfactory → core/mura/resourceBundle/resourceBundleFactory.cfc` | 2 | high | declares getKey(), getResourceBundle() | `core/mura/extend/extendManager.cfc:409` variable 'rbFactory' has no component ref |
| 6 | `rc.homebean → core/mura/content/contentBean.cfc` | 4 | high | declares getContentID(), getURL() | `admin/core/views/carch/frontendconfigurator.cfm:322` variable 'rc.homeBean' has no component ref |
| 6 | `rc.listbean → core/mura/mailinglist/mailinglistBean.cfc` | 1 | high | declares getispurge(), getname(), getisPublic(), getdescription(), getmlid() | `admin/core/views/cmailinglist/edit.cfm:92` variable 'rc.listBean' has no component ref |
| 6 | `tokens → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next(), getCurrentIndex() | `admin/core/views/cusers/edituser.cfm:746` variable 'tokens' has no component ref |
| 6 | `variables.instance.content → core/mura/content/contentBean.cfc` | 15 | medium | declares setIsNew(); named like the receiver 'content' (15 candidates) | `core/mura/content/contentNavBean.cfc:135` variable 'variables.instance.content' has no component ref |
| 6 | `variables.parent → core/mura/bean/beanFactory.cfc` | 1 | medium | declares shutdown() | `core/mura/bean/beanFactory.cfc:332` variable 'variables.parent' has no component ref |
| 5 | `arguments.event → core/mura/bean/bean.cfc` | 4 | low | declares exists() (4 candidates) | `core/mura/Handler/standardEventsHandler.cfc:198` variable 'arguments.event' has no component ref |
| 5 | `arguments.event → core/mura/user/userBean.cfc` | 4 | high | declares getErrors(), getInactive(), getUserID() | `core/mura/Handler/standardEventsHandler.cfc:591` variable 'arguments.event' has no component ref |
| 5 | `assignment` | 1 | none |  | `admin/core/views/cperm/main.cfm:151` variable 'assignment' has no component ref |
| 5 | `beaninstance → core/mura/bean/bean.cfc` | 2 | high | declares getEntityDisplayName(), pluralizeHasRefName() | `admin/core/views/carch/dsp_secondary_menu.cfm:309` variable 'beanInstance' has no component ref |
| 5 | `builtsites[] → core/mura/settings/settingsBean.cfc` | 1 | high | declares getDisplayPoolID(), getRBFactory(), discoverBeans() | `core/mura/settings/settingsManager.cfc:481` variable 'builtSites[]' has no component ref |
| 5 | `cb → core/mura/content/contentBean.cfc` | 2 | high | declares getCrumbArray(), getContentHistID(), getActive(), getLastUpdate(), getStats() | `core/mura/content/contentManager.cfc:2796` variable 'cb' has no component ref |
| 5 | `chain → core/mura/content/approval/approvalChainBean.cfc` | 1 | high | declares getChainID(), getIsNew() | `admin/core/views/cchain/dsp_secondary_menu.cfm:83` variable 'chain' has no component ref |
| 5 | `homebean → core/mura/content/contentBean.cfc` | 6 | low | declares getURL() (6 candidates) | `admin/core/views/carch/dsp_close_compact_display.cfm:96` variable 'homeBean' has no component ref |
| 5 | `inheritbean → core/mura/content/contentBean.cfc` | 4 | high | declares getContentID(), getCrumbArray(), getEditURL(), getMenuTitle() | `admin/core/views/carch/form/dsp_panel_layoutobjects.cfm:169` variable 'inheritBean' has no component ref |
| 5 | `local.connection` | 0 | none |  | `core/mura/content/file/fileManager.cfc:567` variable 'local.connection' has no component ref |
| 5 | `qs → core/mura/bean/beanFeed.cfc` | 3 | low | declares addParam() (3 candidates) | `core/mura/bean/beanORM.cfc:422` variable 'qs' has no component ref |
| 5 | `rbfactory → core/mura/resourceBundle/resourceBundle.cfc` | 1 | high | declares messageFormat(); named like the receiver 'ResourceBundle' | `core/modules/v1/favorites/index.cfm:70` variable 'rbFactory' has no component ref |
| 5 | `rc.contentbean → core/mura/bean/beanExtendable.cfc` | 6 | low | declares getSubType(), getType() (6 candidates) | `admin/core/views/carch/loadrelatedcontent.cfm:97` variable 'rc.contentBean' has no component ref |
| 5 | `rc.it → core/mura/bean/beanFeed.cfc` | 2 | low | declares getPageIndex() (2 candidates) | `admin/core/views/cusers/inc/dsp_nextn.cfm:136` variable 'rc.it' has no component ref |
| 5 | `rc.sitebean → core/mura/settings/settingsBean.cfc` | 26 | high | declares getSiteID(), getExportLocation() | `admin/common/layouts/includes/nav.cfm:280` variable 'rc.siteBean' has no component ref |
| 5 | `subitem → core/mura/bean/bean.cfc` | 13 | low | declares getValue(), getIsNew(), setValue() (2 candidates) | `core/mura/bean/beanORM.cfc:617` variable 'subItem' has no component ref |
| 5 | `variables.commentbean → core/mura/content/contentCommentBean.cfc` | 21 | high | declares setName(), setComments(), setURL(), setEmail(), save() | `core/modules/v1/comments/index.cfm:192` variable 'variables.commentBean' has no component ref |
| 5 | `variables.sitebean → core/mura/settings/settingsBean.cfc` | 1 | high | declares getThemeIncludePath(), getLocalHandler(), getThemeAssetMap() | `core/appcfc/onApplicationStart_include.cfm:826` variable 'variables.siteBean' has no component ref |
| 5 | `variables.utility → core/mura/utility.cfc` | 1 | high | declares getRequestHost(), getRequestProtocol(); named like the receiver 'utility' | `core/modules/v1/login/model/providers/baseLoginProvider.cfc:12` variable 'variables.utility' has no component ref |
| 4 | `arguments.bundle → core/mura/bean/bean.cfc` | 13 | low | declares setValue() (13 candidates) | `core/mura/bean/beanORM.cfc:965` variable 'arguments.bundle' has no component ref |
| 4 | `arguments.collection → core/mura/iterator/queryIterator.cfc` | 1 | high | declares getRecordCount(), hasNext(), next(), getRecordIndex() | `core/mura/MasaScope.cfc:468` variable 'arguments.collection' has no component ref |
| 4 | `arguments.currenteventobject → core/mura/MasaScope.cfc` | 1 | high | declares event(), getValue() | `core/mura/plugin/pluginManager.cfc:1300` variable 'arguments.currentEventObject' has no component ref |
| 4 | `arguments.data.$` | 1 | none |  | `core/tests/specs/mura/core/muraScope.cfc:60` variable 'arguments.data.$' has no component ref |
| 4 | `arguments.entity → core/mura/bean/bean.cfc` | 6 | low | declares getAll() (6 candidates) | `core/mura/client/api/json/v1/jsonApiUtility.cfc:2148` variable 'arguments.entity' has no component ref |
| 4 | `arguments.event → core/mura/category/categoryBean.cfc` | 6 | low | declares getFilename() (6 candidates) | `core/mura/Handler/standardEventsHandler.cfc:355` variable 'arguments.event' has no component ref |
| 4 | `arguments.memberbean → core/mura/mailinglist/memberBean.cfc` | 6 | high | declares getEmail(), getMLID(), getSiteID(); named like the receiver 'memberBean' | `core/mura/mailinglist/memberDAO.cfc:184` variable 'arguments.memberBean' has no component ref |
| 4 | `arguments.obj → core/mura/content/contentBean.cfc` | 26 | high | declares setSiteID(), setContentID(), setContentHistID(), setModuleID() | `core/mura/content/contentBean.cfc:1082` variable 'arguments.obj' has no component ref |
| 4 | `arguments.renderer → core/mura/content/contentNavBean.cfc` | 13 | low | declares getValue() (13 candidates) | `core/mura/content/contentRendererUtility.cfc:670` variable 'arguments.renderer' has no component ref |
| 4 | `arguments.userbean → core/mura/user/userBean.cfc` | 1 | high | declares setPassword(), save(), getAllValues(), getEmail(); named like the receiver 'userBean' | `core/mura/user/userUtility.cfc:465` variable 'arguments.userBean' has no component ref |
| 4 | `attributes.pluginevent → core/mura/cfobject.cfc` | 13 | low | declares setValue() (13 candidates) | `admin/core/views/carch/dsp_nest.cfm:361` variable 'attributes.pluginEvent' has no component ref |
| 4 | `bean → core/mura/cfobject.cfc` | 2 | low | declares invokeMethod() (2 candidates) | `core/mura/content/contentNavBean.cfc:117` variable 'bean' has no component ref |
| 4 | `calendarutility → core/mura/content/contentCalendarUtilityBean.cfc` | 1 | medium | declares getCalendarItems() | `core/modules/v1/collection/index.cfm:148` variable 'calendarUtility' has no component ref |
| 4 | `categorybean` | 6 | none |  | `core/mura/content/contentUtility.cfc:2008` variable 'categoryBean' has no component ref |
| 4 | `config → core/mura/configBean.cfc` | 1 | high | declares getDatasource(); named like the receiver 'config' | `core/mura/content/contentUtility.cfc:186` variable 'config' has no component ref |
| 4 | `conflictdetail → core/mura/content/contentBean.cfc` | 1 | medium | declares getDisplayStart() | `admin/core/views/carch/dsp_status.cfm:182` variable 'conflictdetail' has no component ref |
| 4 | `contentrendererutility → core/mura/content/contentRendererUtility.cfc` | 1 | high | declares renderObjectClassOption(); named like the receiver 'contentRendererUtility' | `admin/core/views/carch/objectclass/legacy/dsp_folders.cfm:112` variable 'contentRendererUtility' has no component ref |
| 4 | `draftversion → core/mura/content/contentBean.cfc` | 1 | high | declares getMenuTitle(), getLastUpdate(), getLastUpdateBy() | `admin/core/views/carch/draftpromptdata.cfm:156` variable 'draftVersion' has no component ref |
| 4 | `event → core/mura/servletEvent.cfc` | 3 | high | declares getContentBean(), getSite() | `core/modules/v1/gallery/index.cfm:116` variable 'event' has no component ref |
| 4 | `image → core/mura/settings/settingsImageSizeBean.cfc` | 26 | high | declares getSiteID(), getSizeID(), getName() | `admin/core/views/csettings/loadcustomimages.cfm:36` variable 'image' has no component ref |
| 4 | `iterator → core/mura/bean/bean.cfc` | 9 | low | declares getEntityName() (9 candidates) | `core/mura/client/api/json/v1/jsonApiUtility.cfc:313` variable 'iterator' has no component ref |
| 4 | `rc.feedbean → core/mura/content/feed/feedBean.cfc` | 15 | high | declares getIsNew(), getType(), getfeedID(), getChannelLink(); named like the receiver 'feedBean' | `admin/core/views/cfeed/dsp_secondary_menu.cfm:91` variable 'rc.feedBean' has no component ref |
| 4 | `rc.newbean → core/mura/content/contentBean.cfc` | 15 | high | declares getIsNew(), getRemotePubDate(), getcontenthistID() | `admin/core/views/cfeed/import1.cfm:130` variable 'rc.newBean' has no component ref |
| 4 | `rc.razunaapi → core/mura/content/file/razuna/razunaAPI.cfc` | 1 | high | declares getFolders(); named like the receiver 'razunaAPI' | `admin/core/controllers/razuna.cfc:45` variable 'rc.razunaAPI' has no component ref |
| 4 | `redirect` | 4 | none |  | `core/mura/user/userUtility.cfc:822` variable 'redirect' has no component ref |
| 4 | `request.contentrenderer → core/mura/content/contentRenderer.cfc` | 4 | medium | declares createHREF(); named like the receiver 'contentRenderer' (4 candidates) | `core/modules/v1/nav/dsp_tag_line.cfm:82` variable 'request.contentRenderer' has no component ref |
| 4 | `request.event → core/mura/event.cfc` | 7 | medium | declares getContentRenderer(); named like the receiver 'event' (7 candidates) | `core/mura/extend/extendData.cfc:447` variable 'request.event' has no component ref |
| 4 | `request.murascope → core/mura/MasaScope.cfc` | 1 | high | declares event(), initTracePoint(), commitTracePoint() | `core/mura/customtags/MuraTracer.cfm:81` variable 'request.muraScope' has no component ref |
| 4 | `ro → core/mura/content/contentCommentBean.cfc` | 10 | low | declares getPrimaryKey(), getValue() (10 candidates) | `core/mura/content/contentDAO.cfc:1699` variable 'ro' has no component ref |
| 4 | `sample → core/mura/content/contentCommentFeedBean.cfc` | 4 | low | declares getTable(), getPrimarykey() (4 candidates) | `core/mura/content/contentGatewayAdobe.cfc:2411` variable 'sample' has no component ref |
| 4 | `sourceextendset → core/mura/extend/extendSet.cfc` | 1 | high | declares getName(), getContainer(), getIsActive(), getOrderNo(), getAttributes() | `core/mura/extend/extendManager.cfc:1632` variable 'sourceExtendSet' has no component ref |
| 4 | `subitems → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next() | `core/mura/bean/beanORM.cfc:615` variable 'subItems' has no component ref |
| 4 | `subtype → core/mura/extend/extendRelatedContentSetBean.cfc` | 3 | low | declares getSubTypeID() (3 candidates) | `core/mura/content/contentManager.cfc:2233` variable 'subtype' has no component ref |
| 4 | `variables.data.$ → core/mura/cfobject.cfc` | 2 | low | declares getServiceFactory(), getBean() (2 candidates) | `core/tests/specs/mura/core/entities.cfc:346` variable 'variables.data.$' has no component ref |
| 4 | `variables.localhandler` | 1 | none |  | `core/appcfc/onApplicationStart_include.cfm:856` variable 'variables.localhandler' has no component ref |
| 4 | `variables.parent` | 2 | none |  | `core/mura/bean/ioc.cfc:700` variable 'variables.parent' has no component ref |
| 3 | `application.classextensionmanager → admin/core/controllers/cextend.cfc` | 2 | low | declares saveAttributeSort() (2 candidates) | `admin/core/controllers/cextend.cfc:217` variable 'application.classExtensionManager' has no component ref |
| 3 | `arguments.child` | 26 | none |  | `core/mura/content/contentBean.cfc:1091` variable 'arguments.child' has no component ref |
| 3 | `arguments.component → core/mura/cfobject.cfc` | 1 | high | declares injectMethod(), setValue() | `core/mura/plugin/pluginConfig.cfc:298` variable 'arguments.component' has no component ref |
| 3 | `arguments.credentials → core/mura/googleAuth.cfc` | 2 | low | declares getKey() (2 candidates) | `core/mura/googleAuth.cfc:41` variable 'arguments.credentials' has no component ref |
| 3 | `arguments.data → core/mura/configBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/category/categoryManager.cfc:235` variable 'arguments.data' has no component ref |
| 3 | `arguments.data → core/mura/user/sessionUserFacade.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/user/userManager.cfc:383` variable 'arguments.data' has no component ref |
| 3 | `arguments.data.$ → core/mura/MasaScope.cfc` | 2 | high | declares announceEvent(), event(), renderEvent() | `core/tests/specs/mura/core/eventHandlers.cfc:23` variable 'arguments.data.$' has no component ref |
| 3 | `arguments.event → core/mura/json.cfc` | 20 | low | declares validate() (20 candidates) | `core/mura/Handler/standardEventsHandler.cfc:269` variable 'arguments.event' has no component ref |
| 3 | `arguments.obj → core/mura/bean/bean.cfc` | 13 | low | declares setValue() (13 candidates) | `core/mura/bean/beanORMVersioned.cfc:85` variable 'arguments.obj' has no component ref |
| 3 | `arguments.rc.item → core/mura/content/contentBean.cfc` | 15 | high | declares getIsNew(), getContentHistID(), getSource() | `admin/core/controllers/carch.cfc:633` variable 'arguments.rc.item' has no component ref |
| 3 | `bean → core/mura/settings/settingsBean.cfc` | 4 | high | declares getIsNew(), getErrors(), getSiteLocale() | `admin/core/controllers/csettings.cfc:198` variable 'bean' has no component ref |
| 3 | `beaninstance` | 14 | none |  | `core/mura/client/api/json/v1/jsonApiUtility.cfc:244` variable 'beanInstance' has no component ref |
| 3 | `bundle → core/mura/settings/settingsBundle.cfc` | 3 | low | declares getBundle() (2 candidates) | `core/mura/publisher.cfc:446` variable 'Bundle' has no component ref |
| 3 | `cache → core/mura/cache/cacheAbstract.cfc` | 4 | low | declares purge() (4 candidates) | `core/mura/content/feed/feedManager.cfc:349` variable 'cache' has no component ref |
| 3 | `cat → core/mura/bean/beanEntity.cfc` | 5 | low | declares getPath() (5 candidates) | `admin/core/views/carch/form/dsp_panel_categories.cfm:198` variable 'cat' has no component ref |
| 3 | `categoryiterator → core/mura/iterator/queryIterator.cfc` | 6 | high | declares setNextN(), hasNext(), next() | `core/mura/content/contentUtility.cfc:2004` variable 'categoryIterator' has no component ref |
| 3 | `childreniterator → core/mura/iterator/queryIterator.cfc` | 6 | high | declares setNextN(), end(), previous() | `core/mura/content/contentUtility.cfc:1761` variable 'childrenIterator' has no component ref |
| 3 | `configbean` | 0 | none |  | `core/modules/v1/login/model/providers/facebookLoginProvider.cfc:11` variable 'configBean' has no component ref |
| 3 | `conflict` | 5 | none |  | `admin/core/views/carch/dsp_status.cfm:178` variable 'conflict' has no component ref |
| 3 | `content → core/mura/content/feed/feedBean.cfc` | 15 | high | declares getIsNew(), getParentID(), getContentID() | `core/mura/client/api/soap/v1/content.cfc:95` variable 'content' has no component ref |
| 3 | `crumb → core/mura/content/contentBean.cfc` | 14 | low | declares getType(), getContentID() (2 candidates) | `core/modules/v1/category_summary/index.cfm:84` variable 'crumb' has no component ref |
| 3 | `destrelatedset → core/mura/extend/extendRelatedContentSetBean.cfc` | 2 | high | declares setAvailableSubTypes(), setOrderNo(), save() | `core/mura/extend/extendManager.cfc:1670` variable 'destRelatedSet' has no component ref |
| 3 | `email → core/mura/mailer.cfc` | 2 | low | declares sendText() (2 candidates) | `core/mura/content/contentCommentBean.cfc:631` variable 'email' has no component ref |
| 3 | `entity → core/mura/bean/beanEntity.cfc` | 3 | low | declares getName(), getDisplayName() (3 candidates) | `admin/core/views/cextend/editrelatedcontentset.cfm:116` variable 'entity' has no component ref |
| 3 | `entity → core/mura/bean/beanExtendable.cfc` | 26 | low | declares getSiteID() (26 candidates) | `core/mura/client/api/json/v1/jsonApiUtility.cfc:3592` variable 'entity' has no component ref |
| 3 | `entity → core/mura/content/contentBean.cfc` | 9 | high | declares getEntityName(), getURL(), getType() | `core/mura/client/api/json/v1/jsonApiUtility.cfc:2092` variable 'entity' has no component ref |
| 3 | `events → core/mura/iterator/queryIterator.cfc` | 15 | high | declares getQuery(), setQuery(), next() | `core/mura/content/contentIntervalManager.cfc:57` variable 'events' has no component ref |
| 3 | `group → core/mura/user/userBean.cfc` | 14 | high | declares getType(), getGroupName(), getFullName() | `admin/core/views/carch/statusmodal.cfm:172` variable 'group' has no component ref |
| 3 | `item → core/mura/bean/bean.cfc` | 9 | low | declares getEntityName(), getPrimaryKey() (5 candidates) | `admin/core/views/carch/loadselectedrelatedcontent.cfm:130` variable 'item' has no component ref |
| 3 | `itembean` | 4 | none |  | `admin/core/views/carch/loadselectedrelatedcontent.cfm:65` variable 'itemBean' has no component ref |
| 3 | `local.beanclass` | 1 | none |  | `core/mura/publisher.cfc:179` variable 'local.beanClass' has no component ref |
| 3 | `members → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next(), getRecordIndex() | `admin/core/views/cchain/pending.cfm:97` variable 'members' has no component ref |
| 3 | `newfeedbean → core/mura/content/contentBean.cfc` | 15 | low | declares getIsNew(), setContentID(), save() (4 candidates) | `core/mura/content/contentUtility.cfc:1790` variable 'newFeedBean' has no component ref |
| 3 | `obj → core/mura/content/contentCommentFeedBean.cfc` | 20 | low | declares validate(), getEntityName() (5 candidates) | `core/mura/content/contentBean.cfc:513` variable 'obj' has no component ref |
| 3 | `obj → core/mura/user/userBean.cfc` | 20 | low | declares validate(), getEntityName() (5 candidates) | `core/mura/user/userBean.cfc:507` variable 'obj' has no component ref |
| 3 | `parentbean → core/mura/bean/beanExtendable.cfc` | 26 | low | declares getSiteID(), getSubType(), getType() (6 candidates) | `admin/core/views/carch/loadnewcontentmenu.cfm:87` variable 'parentBean' has no component ref |
| 3 | `pluginevent → core/mura/content/contentNavBean.cfc` | 13 | low | declares getValue(), setValue() (13 candidates) | `core/mura/content/contentUtility.cfc:1071` variable 'pluginEvent' has no component ref |
| 3 | `rc.items → core/mura/iterator/queryIterator.cfc` | 1 | high | declares next(), currentIndex() | `admin/core/views/carch/hist.cfm:144` variable 'rc.items' has no component ref |
| 3 | `rc.parentbean → core/mura/content/contentBean.cfc` | 1 | high | declares getClassExtension(), requiresApproval() | `admin/core/views/carch/edit.cfm:114` variable 'rc.parentBean' has no component ref |
| 3 | `renderer` | 4 | none |  | `core/mura/Handler/standardEventsHandler.cfc:853` variable 'renderer' has no component ref |
| 3 | `response` | 0 | none |  | `core/mura/content/file/renderLucee.cfc:86` variable 'response' has no component ref |
| 3 | `rssets[] → core/mura/extend/extendRelatedContentSetBean.cfc` | 1 | high | declares getRelatedContentSetID(), getName(), getEntityType() | `core/mura/client/api/json/v1/jsonApiUtility.cfc:3530` variable 'rssets[]' has no component ref |
| 3 | `runtime` | 0 | none |  | `core/mura/cache/cacheSimple.cfc:131` variable 'Runtime' has no component ref |
| 3 | `variables.archive → core/mura/content/contentBean.cfc` | 4 | high | declares getCrumbIterator(), getContentID(), getFilename() | `core/modules/v1/nav/dsp_archive.cfm:85` variable 'variables.archive' has no component ref |
| 3 | `variables.contentgateway → core/mura/dashboard/dashboardManager.cfc` | 2 | low | declares getRecentUpdates() (2 candidates) | `core/mura/dashboard/dashboardManager.cfc:147` variable 'variables.contentGateway' has no component ref |
| 3 | `variables.crumb → core/mura/content/contentBean.cfc` | 14 | low | declares getType(), getContentID() (2 candidates) | `core/modules/v1/nav/calendarNav/index.cfm:100` variable 'variables.crumb' has no component ref |
| 3 | `variables.event → core/mura/event.cfc` | 13 | medium | declares setValue(); named like the receiver 'event' (13 candidates) | `core/modules/v1/component/index.cfm:84` variable 'variables.event' has no component ref |
| 2 | `actions → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next() | `admin/core/views/carch/statusmodal.cfm:183` variable 'actions' has no component ref |
| 2 | `address → core/mura/content/contentBean.cfc` | 45 | low | declares save(), getAllValues() (6 candidates) | `core/mura/client/api/soap/v1/user.cfc:149` variable 'address' has no component ref |
| 2 | `adminuseriterator → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next() | `core/modules/v1/login/model/oauthLoginUtility.cfc:29` variable 'adminUserIterator' has no component ref |
| 2 | `ap → core/mura/content/contentBean.cfc` | 15 | low | declares getIsNew() (15 candidates) | `core/mura/content/contentDAO.cfc:785` variable 'ap' has no component ref |
| 2 | `application.confibean → core/mura/configBean.cfc` | 2 | low | declares getFileDir() (2 candidates) | `core/mura/publisher.cfc:1546` variable 'application.confiBean' has no component ref |
| 2 | `approvalrequest → core/mura/content/approval/approvalRequestBean.cfc` | 15 | medium | declares getIsNew(), getStatus(); named like the receiver 'approvalRequest' (3 candidates) | `admin/core/views/carch/edit.cfm:247` variable 'approvalRequest' has no component ref |
| 2 | `archived → core/mura/content/contentBean.cfc` | 15 | low | declares getIsNew(), getContentID() (4 candidates) | `core/mura/Handler/standardEventsHandler.cfc:473` variable 'archived' has no component ref |
| 2 | `arguments.address → core/mura/user/userBean.cfc` | 26 | low | declares setSiteID(), setUserID() (3 candidates) | `core/mura/user/userBean.cfc:631` variable 'arguments.address' has no component ref |
| 2 | `arguments.applicationscope.pluginmanager → core/mura/plugin/pluginManager.cfc` | 2 | medium | declares announceEvent(); named like the receiver 'pluginManager' (2 candidates) | `core/appcfc/onSessionEnd_include.cfm:107` variable 'arguments.ApplicationScope.pluginManager' has no component ref |
| 2 | `arguments.beaninstance → core/mura/bean/bean.cfc` | 2 | high | declares getEntityDisplayName(), getProperties() | `core/mura/client/api/json/v1/jsonApiUtility.cfc:251` variable 'arguments.beanInstance' has no component ref |
| 2 | `arguments.cache → core/mura/cache/cacheAbstract.cfc` | 4 | low | declares purge() (4 candidates) | `core/mura/content/contentManager.cfc:2896` variable 'arguments.cache' has no component ref |
| 2 | `arguments.child → core/mura/formBuilder/datasetBean.cfc` | 26 | low | declares setSiteID(), setParentID() (2 candidates) | `core/mura/category/categoryBean.cfc:180` variable 'arguments.child' has no component ref |
| 2 | `arguments.context → core/mura/MasaScope.cfc` | 1 | high | declares event(), getValue() | `core/mura/plugin/pluginStandardEventWrapper.cfc:96` variable 'arguments.context' has no component ref |
| 2 | `arguments.displayinterval → core/mura/content/contentBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/content/contentBean.cfc:808` variable 'arguments.displayInterval' has no component ref |
| 2 | `arguments.iterator → core/mura/bean/beanFeed.cfc` | 6 | low | declares setNextN() (6 candidates) | `core/mura/client/api/json/v1/jsonApiUtility.cfc:3165` variable 'arguments.iterator' has no component ref |
| 2 | `arguments.newbean → core/mura/content/contentBean.cfc` | 25 | high | declares getAllValues(), getContentHistID() | `core/mura/content/contentFileMetaDataBean.cfc:139` variable 'arguments.newBean' has no component ref |
| 2 | `arguments.rc.attributebean → admin/core/controllers/cchain.cfc` | 45 | low | declares save() (45 candidates) | `admin/core/controllers/cextend.cfc:204` variable 'arguments.rc.attributeBean' has no component ref |
| 2 | `arguments.rc.changesets → core/mura/iterator/queryIterator.cfc` | 6 | high | declares setNextN(), setPage() | `admin/core/controllers/cchangesets.cfc:140` variable 'arguments.rc.changesets' has no component ref |
| 2 | `arguments.rc.extendsetbean → core/mura/extend/extendSet.cfc` | 45 | medium | declares save(); named like the receiver 'extendSetBean' (45 candidates) | `admin/core/controllers/cextend.cfc:166` variable 'arguments.rc.extendSetBean' has no component ref |
| 2 | `arguments.rc.subtypebean → admin/core/controllers/cchain.cfc` | 45 | low | declares save() (45 candidates) | `admin/core/controllers/cextend.cfc:144` variable 'arguments.rc.subtypeBean' has no component ref |
| 2 | `arguments.renderer → core/mura/settings/settingsBean.cfc` | 1 | medium | declares getWebPath() | `core/mura/content/contentRendererUtility.cfc:463` variable 'arguments.renderer' has no component ref |
| 2 | `attributes.murascope → core/mura/MasaScope.cfc` | 2 | low | declares renderCSRFTokens() (2 candidates) | `admin/core/views/ccategory/dsp_nest.cfm:141` variable 'attributes.muraScope' has no component ref |
| 2 | `attributes.userbean → core/mura/user/userBean.cfc` | 6 | medium | declares getCategoryID(); named like the receiver 'userBean' (6 candidates) | `admin/core/views/ceditprofile/dsp_categories_nest.cfm:97` variable 'attributes.userBean' has no component ref |
| 2 | `atts[i] → core/mura/extend/extendAttribute.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/extend/extendSet.cfc:478` variable 'atts[i]' has no component ref |
| 2 | `bean → core/mura/bean/bean.cfc` | 10 | high | declares has(), getPrimaryKey(); named like the receiver 'bean' | `core/mura/bean/bean.cfc:138` variable 'bean' has no component ref |
| 2 | `bean → core/mura/bean/beanORM.cfc` | 2 | low | declares toBundle() (2 candidates) | `core/mura/settings/settingsBundle.cfc:1898` variable 'bean' has no component ref |
| 2 | `calendars → core/mura/iterator/queryIterator.cfc` | 1 | high | declares next(), currentIndex() | `core/modules/v1/calendar/index.cfm:146` variable 'calendars' has no component ref |
| 2 | `cat → core/mura/category/categoryBean.cfc` | 6 | low | declares getName(), getCategoryID() (4 candidates) | `core/modules/v1/nav/model/beans/catnav.cfc:60` variable 'cat' has no component ref |
| 2 | `conflictdetails → core/mura/iterator/queryIterator.cfc` | 1 | high | declares next(), hasNext() | `admin/core/views/carch/dsp_status.cfm:181` variable 'conflictDetails' has no component ref |
| 2 | `conflicts → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next() | `admin/core/views/carch/dsp_status.cfm:171` variable 'conflicts' has no component ref |
| 2 | `content → core/mura/client/api/soap/v1/content.cfc` | 4 | low | declares deleteVersion(); named like the receiver 'content' (4 candidates) | `core/mura/client/api/soap/v1/content.cfc:189` variable 'content' has no component ref |
| 2 | `current → core/mura/bean/beanFeed.cfc` | 8 | low | declares setSortBy(), setSortDirection() (8 candidates) | `admin/core/views/carch/list.cfm:597` variable 'current' has no component ref |
| 2 | `databean → core/mura/bean/beanORM.cfc` | 10 | low | declares getPrimaryKey(), loadby() (8 candidates) | `core/mura/formBuilder/formBuilderManager.cfc:311` variable 'dataBean' has no component ref |
| 2 | `event → core/mura/configBean.cfc` | 2 | low | declares getAssetPath() (2 candidates) | `core/modules/v1/feedslideshow/index.cfm:80` variable 'event' has no component ref |
| 2 | `extendset → core/mura/extend/extendSet.cfc` | 1 | high | declares getAttributes(), getName(), getAttributeBean(); named like the receiver 'extendSet' | `admin/core/views/cextend/editattributes.cfm:80` variable 'extendSet' has no component ref |
| 2 | `filemetadata → core/mura/MasaScope.cfc` | 3 | low | declares generateCSRFTokens(), renderCSRFTokens() (2 candidates) | `core/mura/customtags/filetools.cfm:80` variable 'fileMetaData' has no component ref |
| 2 | `form` | 0 | none |  | `core/mura/content/dataCollection/dataCollectionBean.cfc:119` variable 'form' has no component ref |
| 2 | `hasmany → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next() | `core/mura/bean/bean.cfc:382` variable 'hasMany' has no component ref |
| 2 | `item → core/mura/cfobject.cfc` | 13 | low | declares getValue() (13 candidates) | `core/mura/MasaScope.cfc:533` variable 'item' has no component ref |
| 2 | `keys → core/mura/configBean.cfc` | 2 | low | declares getMode() (2 candidates) | `core/mura/publisher.cfc:2559` variable 'keys' has no component ref |
| 2 | `lhttpservice` | 0 | none |  | `core/modules/v1/collection/layout/index.cfm:143` variable 'lhttpService' has no component ref |
| 2 | `local.beanclass → core/mura/bean/bean.cfc` | 1 | high | declares hasProperty(), getTable() | `core/mura/settings/settingsDAO.cfc:306` variable 'local.beanClass' has no component ref |
| 2 | `local.stats → admin/core/controllers/cchain.cfc` | 45 | low | declares save() (45 candidates) | `admin/core/controllers/carch.cfc:605` variable 'local.stats' has no component ref |
| 2 | `local.tmppart` | 0 | none |  | `core/mura/content/file/fileManager.cfc:739` variable 'local.tmpPart' has no component ref |
| 2 | `localobject → core/mura/bean/beanORM.cfc` | 10 | low | declares getPrimaryKey(), loadBy() (8 candidates) | `core/mura/bean/beanRemotePointer.cfc:10` variable 'localObject' has no component ref |
| 2 | `member → core/mura/content/approval/approvalChainMembershipBean.cfc` | 3 | high | declares getGroup(), getPendingContentIterator() | `admin/core/views/cchain/pending.cfm:101` variable 'member' has no component ref |
| 2 | `newattribute → core/mura/content/contentBean.cfc` | 26 | low | declares setSiteID(), setOrderno() (5 candidates) | `admin/core/views/cextend/editattributes.cfm:116` variable 'newAttribute' has no component ref |
| 2 | `obj → core/mura/bean/beanEntity.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/bean/beanORM.cfc:464` variable 'obj' has no component ref |
| 2 | `obj → core/mura/category/categoryBean.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/category/categoryManager.cfc:306` variable 'obj' has no component ref |
| 2 | `obj → core/mura/content/contentBean.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/content/contentCommentBean.cfc:536` variable 'obj' has no component ref |
| 2 | `object → core/mura/cfobject.cfc` | 2 | low | declares invokeMethod() (2 candidates) | `core/mura/MasaScope.cfc:135` variable 'object' has no component ref |
| 2 | `parent` | 1 | none |  | `core/mura/content/contentManager.cfc:1095` variable 'parent' has no component ref |
| 2 | `previous → core/mura/content/contentBean.cfc` | 15 | high | declares getIsNew(), deleteVersion() | `core/mura/content/changeset/changesetManager.cfc:539` variable 'previous' has no component ref |
| 2 | `publicuseriterator → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next() | `core/modules/v1/login/model/oauthLoginUtility.cfc:35` variable 'publicUserIterator' has no component ref |
| 2 | `razunasettings → core/mura/content/file/razuna/razunaSettingsBean.cfc` | 1 | high | declares getAPIKey(), getHostName(); named like the receiver 'razunaSettings' | `core/mura/customtags/fileselector.cfm:68` variable 'razunaSettings' has no component ref |
| 2 | `rbfactory → core/modules/v1/filebrowser/model/beans/filebrowser.cfc` | 3 | low | declares getResourceBundle() (3 candidates) | `core/modules/v1/favorites/index.cfm:70` variable 'rbFactory' has no component ref |
| 2 | `rc.contentbean → admin/core/controllers/cchain.cfc` | 45 | low | declares save() (45 candidates) | `admin/core/views/carch/variationtargeting.cfm:7` variable 'rc.contentBean' has no component ref |
| 2 | `rc.parentbean → core/mura/bean/beanExtendable.cfc` | 14 | low | declares getType() (14 candidates) | `admin/core/views/carch/dsp_status.cfm:160` variable 'rc.parentBean' has no component ref |
| 2 | `rc.versionbean → core/mura/content/contentBean.cfc` | 1 | high | declares getContentHistID(), getContentID() | `admin/core/views/carch/updateobjectparams.cfm:76` variable 'rc.versionBean' has no component ref |
| 2 | `redirect → core/mura/user/userRedirectBean.cfc` | 4 | high | declares exists(), getURL(), apply() | `core/mura/content/contentServer.cfc:241` variable 'redirect' has no component ref |
| 2 | `redirectchecker → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next() | `core/mura/user/userUtility.cfc:907` variable 'redirectChecker' has no component ref |
| 2 | `related → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next() | `admin/core/views/carch/loadselectedrelatedcontent.cfm:125` variable 'related' has no component ref |
| 2 | `relatedcontentbean → core/mura/content/contentBean.cfc` | 15 | low | declares getIsNew(), getContentID() (4 candidates) | `core/mura/content/contentUtility.cfc:1837` variable 'relatedContentBean' has no component ref |
| 2 | `request.muraglobalevent → core/mura/configBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `admin/Application.cfc:337` variable 'request.muraGlobalEvent' has no component ref |
| 2 | `requests → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next() | `admin/core/views/cchain/pending.cfm:104` variable 'requests' has no component ref |
| 2 | `returnstruct.sitetemplate → core/mura/settings/settingsBean.cfc` | 1 | high | declares discoverGlobalModules(), discoverGlobalContentTypes() | `core/mura/settings/settingsManager.cfc:435` variable 'returnStruct.siteTemplate' has no component ref |
| 2 | `services → core/mura/iterator/queryIterator.cfc` | 1 | high | declares hasNext(), next() | `admin/core/views/cwebservice/list.cfm:97` variable 'services' has no component ref |
| 2 | `session.mura.editbean → core/mura/bean/beanEntity.cfc` | 12 | low | declares getLastUpdate(), setLastUpdate() (12 candidates) | `admin/core/views/carch/update.cfm:104` variable 'session.mura.editBean' has no component ref |
| 2 | `sites[] → core/mura/content/contentRenderer.cfc` | 3 | low | declares registerDisplayObject() (3 candidates) | `core/appcfc/onApplicationStart_include.cfm:985` variable 'sites[]' has no component ref |
| 2 | `sites[] → core/mura/settings/settingsBean.cfc` | 2 | low | declares getAccessControlOriginDomainList() (2 candidates) | `core/mura/settings/settingsManager.cfc:860` variable 'sites[]' has no component ref |
| 2 | `sourcerelatedset → core/mura/extend/extendRelatedContentSetBean.cfc` | 2 | high | declares getName(), getAvailableSubTypes(), getOrderNo() | `core/mura/extend/extendManager.cfc:1670` variable 'sourceRelatedSet' has no component ref |
| 2 | `subitem` | 15 | none |  | `core/mura/bean/beanORM.cfc:717` variable 'subitem' has no component ref |
| 2 | `sys.out` | 1 | none |  | `core/mura/bean/ioc.cfc:654` variable 'sys.out' has no component ref |
| 2 | `targetcontent → core/mura/content/contentBean.cfc` | 14 | high | declares getType(), getObjectParam() | `core/modules/v1/nav/calendarNav/index.cfm:113` variable 'targetContent' has no component ref |
| 2 | `user` | 4 | none |  | `core/mura/user/userUtility.cfc:827` variable 'user' has no component ref |
| 2 | `variables.contentgateway → core/mura/content/contentBean.cfc` | 6 | low | declares getKids() (6 candidates) | `core/mura/content/contentManager.cfc:2713` variable 'variables.contentGateway' has no component ref |
| 2 | `variables.crumbiterator → core/mura/iterator/queryIterator.cfc` | 1 | medium | declares next() | `core/modules/v1/nav/calendarNav/index.cfm:99` variable 'variables.crumbIterator' has no component ref |
| 2 | `variables.data.$ → core/mura/bean/beanFactory.cfc` | 4 | low | declares containsBean() (3 candidates) | `core/tests/specs/mura/core/entities.cfc:346` variable 'variables.data.$' has no component ref |
| 2 | `variables.dbutility → core/mura/dbUtility.cfc` | 1 | high | declares transformParamType(); named like the receiver 'dbUtility' | `core/mura/bean/beanFeed.cfc:332` variable 'variables.dbUtility' has no component ref |
| 2 | `variables.eventhandler → core/mura/plugin/pluginStandardEventWrapper.cfc` | 1 | medium | declares handle() | `core/mura/plugin/pluginStandardEventWrapper.cfc:122` variable 'variables.eventHandler' has no component ref |
| 2 | `variables.parent → core/mura/bean/ioc.cfc` | 21 | low | declares getBean() (20 candidates) | `core/mura/bean/ioc.cfc:212` variable 'variables.parent' has no component ref |
| 2 | `variables.parentfactory → core/mura/resourceBundle/resourceBundle.cfc` | 2 | low | declares getKeyValue() (2 candidates) | `core/mura/resourceBundle/resourceBundleFactory.cfc:127` variable 'variables.parentFactory' has no component ref |
| 2 | `variables.rbfactory → core/modules/v1/filebrowser/model/beans/filebrowser.cfc` | 3 | low | declares getResourceBundle() (3 candidates) | `core/modules/v1/nav/dsp_tag_cloud.cfm:133` variable 'variables.rbFactory' has no component ref |
| 2 | `variables.rbfactory → core/mura/resourceBundle/resourceBundle.cfc` | 1 | high | declares messageFormat(); named like the receiver 'ResourceBundle' | `core/modules/v1/nav/dsp_tag_cloud.cfm:133` variable 'variables.rbFactory' has no component ref |
| 2 | `variables.sectionbean → core/mura/configBean.cfc` | 2 | low | declares getTitle() (2 candidates) | `core/modules/v1/search/index.cfm:150` variable 'variables.sectionBean' has no component ref |
| 2 | `variables.userbean → core/mura/user/userBean.cfc` | 15 | medium | declares getIsNew(), setSiteID(); named like the receiver 'userBean' (12 candidates) | `core/mura/user/sessionUserFacade.cfc:94` variable 'variables.userBean' has no component ref |
| 2 | `variationtargeting → core/mura/content/contentVariationTargetingBean.cfc` | 2 | high | declares getInitJS(), getTargetingJS() | `core/mura/Handler/standardEventsHandler.cfc:1008` variable 'variationTargeting' has no component ref |
| 2 | `version → core/mura/content/contentBean.cfc` | 1 | high | declares getContentHistID(), getSiteID() | `core/mura/content/contentManager.cfc:2946` variable 'version' has no component ref |
| 1 | `action → core/mura/user/userBean.cfc` | 2 | medium | declares getFullName(); named like the receiver 'User' (2 candidates) | `admin/core/views/carch/statusmodal.cfm:193` variable 'action' has no component ref |
| 1 | `activebean → core/mura/content/contentBean.cfc` | 2 | low | declares getURLtitle() (2 candidates) | `core/mura/content/contentManager.cfc:1351` variable 'activeBean' has no component ref |
| 1 | `address → core/mura/user/addressBean.cfc` | 45 | medium | declares save(); named like the receiver 'address' (45 candidates) | `core/mura/user/userBean.cfc:574` variable 'address' has no component ref |
| 1 | `admingroup → core/mura/content/contentCommentBean.cfc` | 4 | low | declares getUserID() (4 candidates) | `admin/core/views/cperm/main.cfm:128` variable 'adminGroup' has no component ref |
| 1 | `application.classextensionmanager → core/mura/extend/extendRelatedContentSetBean.cfc` | 2 | low | declares getAvailableSubTypes() (2 candidates) | `admin/core/views/carch/loadnewcontentmenu.cfm:87` variable 'application.classExtensionManager' has no component ref |
| 1 | `application.classextensionmanager → core/mura/extend/extendSet.cfc` | 2 | medium | declares getattributeBean(); named like the receiver 'ExtendSetBean' (2 candidates) | `admin/core/controllers/cextend.cfc:200` variable 'application.classExtensionManager' has no component ref |
| 1 | `arguments.changesetbean → core/mura/content/changeset/changesetBean.cfc` | 2 | medium | declares getChangesetID(); named like the receiver 'changesetBean' (2 candidates) | `core/mura/publisher.cfc:437` variable 'arguments.changesetBean' has no component ref |
| 1 | `arguments.contentbean → core/mura/content/contentCommentFeedBean.cfc` | 26 | low | declares getSiteID() (26 candidates) | `core/mura/content/approval/approvalRequestBean.cfc:162` variable 'arguments.contentBean' has no component ref |
| 1 | `arguments.contentbean → core/mura/formBuilder/datasetBean.cfc` | 26 | low | declares getSiteID() (26 candidates) | `core/mura/formBuilder/formBuilderManager.cfc:535` variable 'arguments.contentBean' has no component ref |
| 1 | `arguments.data → core/mura/content/contentBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/content/feed/feedManager.cfc:425` variable 'arguments.data' has no component ref |
| 1 | `arguments.data → core/mura/settings/settingsBundle.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/settings/settingsManager.cfc:815` variable 'arguments.data' has no component ref |
| 1 | `arguments.data.bean → core/mura/content/feed/feedBean.cfc` | 15 | medium | declares getQuery(); named like the receiver 'Feed' (15 candidates) | `core/tests/specs/mura/core/entities.cfc:378` variable 'arguments.data.bean' has no component ref |
| 1 | `arguments.event → core/mura/servletEvent.cfc` | 1 | medium | declares getScope() | `core/templates/lockdown.cfm:193` variable 'arguments.event' has no component ref |
| 1 | `arguments.iterator → core/mura/bean/bean.cfc` | 9 | low | declares getEntityName() (9 candidates) | `core/mura/client/api/json/v1/jsonApiUtility.cfc:2923` variable 'arguments.iterator' has no component ref |
| 1 | `arguments.keyfactory → core/mura/configBean.cfc` | 2 | low | declares getMode() (2 candidates) | `core/mura/publisher.cfc:1683` variable 'arguments.keyFactory' has no component ref |
| 1 | `arguments.loginobject → core/mura/login/loginManager.cfc` | 6 | low | declares login() (6 candidates) | `core/mura/login/loginManager.cfc:386` variable 'arguments.loginObject' has no component ref |
| 1 | `arguments.object → core/mura/bean/beanExtendable.cfc` | 26 | low | declares getSiteID() (26 candidates) | `core/mura/bean/beanValidator.cfc:245` variable 'arguments.object' has no component ref |
| 1 | `arguments.parentbean → core/mura/content/contentBean.cfc` | 2 | low | declares getTitle() (2 candidates) | `core/mura/content/contentUtility.cfc:899` variable 'arguments.parentBean' has no component ref |
| 1 | `arguments.qs → core/mura/bean/beanFeed.cfc` | 3 | low | declares addParam() (3 candidates) | `core/mura/bean/beanORM.cfc:359` variable 'arguments.qs' has no component ref |
| 1 | `arguments.rc.contentbean → core/mura/bean/bean.cfc` | 4 | low | declares getErrors() (4 candidates) | `admin/core/controllers/carch.cfc:391` variable 'arguments.rc.contentBean' has no component ref |
| 1 | `arguments.rc.contentbean → core/mura/content/contentBean.cfc` | 1 | high | declares getStats(); named like the receiver 'contentBean' | `admin/core/views/carch/frontendconfigurator.cfm:289` variable 'arguments.rc.contentBean' has no component ref |
| 1 | `arguments.rc.contentbean → core/mura/iterator/queryIterator.cfc` | 1 | medium | declares hasNext() | `admin/core/controllers/carch.cfc:416` variable 'arguments.rc.contentBean' has no component ref |
| 1 | `arguments.rc.rcsbean → admin/core/controllers/cchain.cfc` | 45 | low | declares save() (45 candidates) | `admin/core/controllers/cextend.cfc:190` variable 'arguments.rc.rcsBean' has no component ref |
| 1 | `arguments.renderer → core/mura/content/contentBean.cfc` | 15 | low | declares getIsNew() (15 candidates) | `core/mura/content/contentRendererUtility.cfc:645` variable 'arguments.renderer' has no component ref |
| 1 | `arguments.renderer → core/mura/event.cfc` | 13 | medium | declares getValue(); named like the receiver 'Event' (13 candidates) | `core/mura/content/contentRendererUtility.cfc:645` variable 'arguments.renderer' has no component ref |
| 1 | `arguments.sourcecontentbean → core/mura/content/contentBean.cfc` | 1 | medium | declares getCategoriesIterator() | `core/mura/content/contentUtility.cfc:2000` variable 'arguments.sourceContentBean' has no component ref |
| 1 | `arguments.version1 → core/mura/content/contentBean.cfc` | 1 | medium | declares getContentHistID() | `core/mura/content/contentDAO.cfc:1689` variable 'arguments.version1' has no component ref |
| 1 | `attributes.cachefactory → core/mura/cache/cacheAbstract.cfc` | 4 | low | declares has(), purge() (4 candidates) | `core/mura/customtags/CacheOMatic.cfm:111` variable 'attributes.cacheFactory' has no component ref |
| 1 | `attributes.feedbean → core/mura/content/feed/feedBean.cfc` | 6 | medium | declares getCategoryID(); named like the receiver 'feedBean' (6 candidates) | `admin/core/views/cfeed/dsp_categories_nest.cfm:97` variable 'attributes.feedBean' has no component ref |
| 1 | `attributes.newbean → core/mura/content/contentBean.cfc` | 1 | medium | declares getcontentHistID() | `admin/core/views/cfeed/dsp_categories_import_nest.cfm:98` variable 'attributes.newBean' has no component ref |
| 1 | `attributes.rc.$ → admin/Application.cfc` | 2 | low | declares rbKey() (2 candidates) | `admin/core/views/ceditprofile/dsp_categories_nest.cfm:103` variable 'attributes.rc.$' has no component ref |
| 1 | `cache` | 0 | none |  | `core/mura/cache/provider/cacheAdobe.cfc:138` variable 'cache' has no component ref |
| 1 | `calendar → core/mura/category/categoryBean.cfc` | 6 | low | declares getFilename() (6 candidates) | `admin/core/views/carch/dsp_status.cfm:182` variable 'calendar' has no component ref |
| 1 | `categorybean → core/mura/category/categoryBean.cfc` | 6 | medium | declares getCategoryID(); named like the receiver 'categoryBean' (6 candidates) | `core/mura/content/contentServer.cfc:567` variable 'categoryBean' has no component ref |
| 1 | `categorynav[ct].nav → core/mura/bean/beanFeed.cfc` | 10 | low | declares getIterator() (10 candidates) | `core/modules/v1/nav/model/beans/catnav.cfc:45` variable 'categorynav[ct].nav' has no component ref |
| 1 | `cats → core/mura/iterator/queryIterator.cfc` | 1 | medium | declares next() | `admin/core/views/carch/form/dsp_panel_categories.cfm:197` variable 'cats' has no component ref |
| 1 | `childcontentbean → core/mura/content/contentBean.cfc` | 4 | low | declares getContentID() (4 candidates) | `core/mura/content/contentUtility.cfc:1767` variable 'childContentBean' has no component ref |
| 1 | `commenter → core/mura/bean/beanORM.cfc` | 15 | low | declares loadBy() (15 candidates) | `core/mura/Handler/standardEventsHandler.cfc:190` variable 'commenter' has no component ref |
| 1 | `content → core/mura/bean/beanExtendable.cfc` | 14 | low | declares getType() (14 candidates) | `admin/core/views/carch/loadquickedit.cfm:141` variable 'content' has no component ref |
| 1 | `content → core/mura/bean/beanFeed.cfc` | 15 | low | declares getQuery() (15 candidates) | `core/mura/client/api/soap/v1/content.cfc:218` variable 'content' has no component ref |
| 1 | `content → core/mura/configBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `admin/core/views/carch/loadquickedit.cfm:139` variable 'content' has no component ref |
| 1 | `content → core/mura/formBuilder/datarecordBean.cfc` | 13 | low | declares getValue() (13 candidates) | `core/mura/formBuilder/formBuilderManager.cfc:537` variable 'content' has no component ref |
| 1 | `contentbean → core/mura/content/feed/feedBean.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/content/feed/feedUtility.cfc:204` variable 'contentBean' has no component ref |
| 1 | `contentstats → core/mura/configBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/modules/v1/comments/ajax/commentsProxy.cfc:111` variable 'contentStats' has no component ref |
| 1 | `context` | 0 | none |  | `core/mura/content/file/renderLucee.cfc:85` variable 'context' has no component ref |
| 1 | `crumbiterator → core/mura/iterator/queryIterator.cfc` | 1 | medium | declares next() | `core/modules/v1/category_summary/index.cfm:83` variable 'crumbIterator' has no component ref |
| 1 | `current → core/mura/content/contentBean.cfc` | 1 | medium | declares getContentHistID() | `core/mura/content/changeset/changesetManager.cfc:547` variable 'current' has no component ref |
| 1 | `data[] → core/mura/cfobject.cfc` | 1 | medium | declares getAsStruct() | `core/mura/cfobject.cfc:216` variable 'data[]' has no component ref |
| 1 | `databean → core/mura/MasaScope.cfc` | 11 | low | declares getFeed() (11 candidates) | `core/mura/formBuilder/formBuilderManager.cfc:312` variable 'dataBean' has no component ref |
| 1 | `databean → core/mura/content/feed/feedBean.cfc` | 15 | medium | declares getQuery(); named like the receiver 'Feed' (15 candidates) | `core/mura/formBuilder/formBuilderManager.cfc:312` variable 'dataBean' has no component ref |
| 1 | `encoder` | 0 | none |  | `core/mura/backport/esapiencode.cfm:48` variable 'encoder' has no component ref |
| 1 | `entities → core/mura/iterator/queryIterator.cfc` | 1 | medium | declares next() | `admin/core/views/cextend/editrelatedcontentset.cfm:114` variable 'entities' has no component ref |
| 1 | `entries` | 0 | none |  | `core/mura/Zip.cfc:796` variable 'entries' has no component ref |
| 1 | `extenddata → core/mura/extend/extendData.cfc` | 1 | high | declares getAllExtendSetData(); named like the receiver 'extendData' | `core/mura/client/api/json/v1/jsonApiUtility.cfc:2141` variable 'extendData' has no component ref |
| 1 | `extendsets[] → core/mura/extend/extendAttribute.cfc` | 3 | low | declares getExtendSetID() (3 candidates) | `core/mura/extend/extendSubType.cfc:975` variable 'extendsets[]' has no component ref |
| 1 | `feed → core/mura/bean/bean.cfc` | 4 | low | declares getTable() (4 candidates) | `core/mura/client/api/json/v1/jsonApiUtility.cfc:3045` variable 'feed' has no component ref |
| 1 | `feed → core/mura/configBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/client/api/soap/v1/feed.cfc:89` variable 'feed' has no component ref |
| 1 | `feedbean → core/mura/content/feed/feedBean.cfc` | 1 | high | declares getDisplayRatings(), getName(); named like the receiver 'feedBean' | `core/modules/v1/feedslideshow/index.cfm:236` variable 'feedBean' has no component ref |
| 1 | `handler → core/modules/v1/filebrowser/model/handlers/handler.cfc` | 1 | high | declares onApplicationLoad(); named like the receiver 'handler' | `core/appcfc/onRequestStart_include.cfm:164` variable 'handler' has no component ref |
| 1 | `head` | 0 | none |  | `core/mura/bean/ioc.cfc:682` variable 'head' has no component ref |
| 1 | `head.listener` | 2 | none |  | `core/mura/bean/ioc.cfc:684` variable 'head.listener' has no component ref |
| 1 | `history → core/mura/iterator/queryIterator.cfc` | 1 | medium | declares next() | `core/mura/content/contentManager.cfc:2945` variable 'history' has no component ref |
| 1 | `item → core/mura/user/userBean.cfc` | 1 | medium | declares getGroupName() | `admin/core/views/cchain/edit.cfm:74` variable 'item' has no component ref |
| 1 | `iterator → core/mura/utility.cfc` | 7 | low | declares getNextN() (7 candidates) | `core/modules/v1/collection/layouts/list/index.cfm:97` variable 'iterator' has no component ref |
| 1 | `j2eesession` | 0 | none |  | `core/mura/login/loginManager.cfc:504` variable 'j2eeSession' has no component ref |
| 1 | `kid → core/mura/category/categoryBean.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/category/categoryBean.cfc:172` variable 'kid' has no component ref |
| 1 | `local.nestedform → core/mura/content/contentBean.cfc` | 1 | medium | declares getBody() | `core/modules/v1/formbuilder/dsp_form.cfm:108` variable 'local.nestedForm' has no component ref |
| 1 | `local.parentbean → core/mura/content/contentCommentFeedBean.cfc` | 26 | low | declares setSiteID() (26 candidates) | `core/mura/content/contentDAO.cfc:1294` variable 'local.parentBean' has no component ref |
| 1 | `local.parentbean → core/mura/content/feed/feedBean.cfc` | 2 | low | declares setParentID() (2 candidates) | `core/mura/content/contentDAO.cfc:1294` variable 'local.parentBean' has no component ref |
| 1 | `local.related → core/mura/bean/bean.cfc` | 11 | low | declares getFeed() (11 candidates) | `core/mura/bean/beanFeed.cfc:1041` variable 'local.related' has no component ref |
| 1 | `local.related → core/mura/bean/beanFeed.cfc` | 1 | medium | declares loadTableMetaData() | `core/mura/bean/beanFeed.cfc:1041` variable 'local.related' has no component ref |
| 1 | `mac` | 0 | none |  | `core/mura/cryptUtility.cfc:160` variable 'mac' has no component ref |
| 1 | `member → core/mura/user/userBean.cfc` | 1 | medium | declares getGroupName() | `admin/core/views/cchain/pending.cfm:101` variable 'member' has no component ref |
| 1 | `newcategorybean → core/mura/content/changeset/changesetBean.cfc` | 6 | low | declares getCategoryID() (6 candidates) | `core/mura/content/contentUtility.cfc:2009` variable 'newCategoryBean' has no component ref |
| 1 | `newcontentbean` | 15 | none |  | `core/mura/content/contentUtility.cfc:1618` variable 'newContentBean' has no component ref |
| 1 | `out` | 0 | none |  | `core/mura/content/file/renderLucee.cfc:90` variable 'out' has no component ref |
| 1 | `rc.content → core/mura/content/contentBean.cfc` | 2 | medium | declares getCrumbArray(); named like the receiver 'content' (2 candidates) | `admin/core/controllers/ccomments.cfc:293` variable 'rc.content' has no component ref |
| 1 | `rc.contentbean → core/mura/extend/extendData.cfc` | 1 | medium | declares getAttributesByType() | `admin/core/views/carch/imagedetails.cfm:15` variable 'rc.contentBean' has no component ref |
| 1 | `rc.extendsetbean → core/mura/extend/extendSet.cfc` | 6 | medium | declares getCategoryID(); named like the receiver 'extendSetBean' (6 candidates) | `admin/core/views/cextend/dsp_categories_nest.cfm:97` variable 'rc.extendSetBean' has no component ref |
| 1 | `rc.parentbean → core/mura/extend/extendRelatedContentSetBean.cfc` | 2 | low | declares getAvailableSubTypes() (2 candidates) | `admin/core/views/carch/edit.cfm:114` variable 'rc.parentBean' has no component ref |
| 1 | `rc.sitebean → core/mura/content/contentRenderer.cfc` | 1 | high | declares useLayoutManager(); named like the receiver 'ContentRenderer' | `admin/core/views/csettings/editsite.cfm:614` variable 'rc.siteBean' has no component ref |
| 1 | `rc.subtypebean → core/mura/extend/extendRelatedContentSetBean.cfc` | 3 | low | declares getSubTypeID() (3 candidates) | `admin/core/controllers/cextend.cfc:154` variable 'rc.subtypeBean' has no component ref |
| 1 | `rc.theimport.parentbean → core/mura/content/contentBean.cfc` | 4 | low | declares getcontentID() (4 candidates) | `admin/core/views/cfeed/import2.cfm:92` variable 'rc.theImport.parentBean' has no component ref |
| 1 | `rc.userbean → core/mura/user/userBean.cfc` | 1 | high | declares getgroupname(); named like the receiver 'userBean' | `admin/core/views/cusers/editgroupmembers.cfm:113` variable 'rc.userBean' has no component ref |
| 1 | `redirectbean → core/mura/user/userBean.cfc` | 4 | low | declares getUserID() (4 candidates) | `core/mura/user/userDAO.cfc:496` variable 'redirectBean' has no component ref |
| 1 | `relatedcontentsetarray[s] → core/mura/extend/extendAttribute.cfc` | 15 | low | declares setIsNew() (15 candidates) | `core/mura/extend/extendSubType.cfc:584` variable 'relatedContentSetArray[s]' has no component ref |
| 1 | `relatedsetbean → core/mura/extend/extendAttribute.cfc` | 4 | low | declares getAsXML() (4 candidates) | `core/mura/extend/extendSubType.cfc:985` variable 'relatedSetBean' has no component ref |
| 1 | `renderer → core/mura/cfobject.cfc` | 1 | medium | declares injectMethod() | `core/mura/Handler/standardEventsHandler.cfc:284` variable 'renderer' has no component ref |
| 1 | `request.contentrenderer → core/mura/cfobject.cfc` | 13 | low | declares getValue() (13 candidates) | `core/mura/MasaScope.cfc:156` variable 'request.contentRenderer' has no component ref |
| 1 | `request.context.$ → core/mura/content/contentFileMetaDataBean.cfc` | 3 | low | declares getURLForImage() (3 candidates) | `core/mura/customtags/filetools.cfm:37` variable 'request.context.$' has no component ref |
| 1 | `request.muracustomimageiterator → core/mura/iterator/queryIterator.cfc` | 1 | medium | declares reset() | `core/mura/client/api/json/v1/jsonApiUtility.cfc:3583` variable 'request.muraCustomImageIterator' has no component ref |
| 1 | `responseobject → core/mura/email/emailBean.cfc` | 2 | low | declares setStatus() (2 candidates) | `core/mura/client/api/feed/v1/feedApiUtility.cfc:175` variable 'responseObject' has no component ref |
| 1 | `retrieved → core/mura/trash/trashItemBean.cfc` | 10 | low | declares setAllValues() (10 candidates) | `core/mura/trash/trashManager.cfc:315` variable 'retrieved' has no component ref |
| 1 | `sample → core/mura/bean/bean.cfc` | 1 | medium | declares getListViewProps() | `admin/core/views/carch/loadselectedrelatedcontent.cfm:131` variable 'sample' has no component ref |
| 1 | `servlet` | 1 | none |  | `core/mura/content/contentServer.cfc:874` variable 'servlet' has no component ref |
| 1 | `session.mura.editbean → core/mura/content/contentBean.cfc` | 1 | medium | declares getContentHistID() | `admin/core/views/carch/loadextendedattributes.cfm:83` variable 'session.mura.editBean' has no component ref |
| 1 | `session.mura.editbean → core/mura/content/contentCommentBean.cfc` | 4 | low | declares getUserID() (4 candidates) | `admin/core/views/cusers/loadextendedattributes.cfm:81` variable 'session.mura.editBean' has no component ref |
| 1 | `sessiondata.plugins[] → core/mura/plugin/pluginApplication.cfc` | 1 | medium | declares setPluginConfig() | `core/mura/plugin/pluginConfig.cfc:284` variable 'sessionData.plugins[]' has no component ref |
| 1 | `sets[i] → core/mura/extend/extendAttribute.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/extend/extendSubType.cfc:930` variable 'sets[i]' has no component ref |
| 1 | `siterenderer` | 0 | none |  | `core/mura/Translator/standardHTMLTranslator.cfc:112` variable 'siteRenderer' has no component ref |
| 1 | `siterenderer → core/mura/cfobject.cfc` | 1 | medium | declares injectMethod() | `core/mura/MasaScope.cfc:182` variable 'siteRenderer' has no component ref |
| 1 | `sitetemplate → core/mura/settings/settingsBean.cfc` | 1 | medium | declares getRBFactory() | `core/mura/settings/settingsManager.cfc:465` variable 'siteTemplate' has no component ref |
| 1 | `st → core/mura/extend/extendManager.cfc` | 2 | low | declares getIconClass() (2 candidates) | `admin/core/views/carch/loadselectedrelatedcontent.cfm:92` variable 'st' has no component ref |
| 1 | `stats → admin/core/controllers/cchain.cfc` | 45 | low | declares save() (45 candidates) | `admin/core/views/carch/frontendconfigurator.cfm:291` variable 'stats' has no component ref |
| 1 | `subitem → core/mura/cfobject.cfc` | 1 | medium | declares getAsStruct() | `core/mura/cfobject.cfc:223` variable 'subItem' has no component ref |
| 1 | `theimport.parentbean → core/mura/content/feed/feedBean.cfc` | 15 | low | declares getIsNew() (15 candidates) | `core/mura/content/feed/feedUtility.cfc:111` variable 'theImport.ParentBean' has no component ref |
| 1 | `themerenderer → core/mura/cfobject.cfc` | 1 | medium | declares injectMethod() | `core/mura/Handler/standardEventsHandler.cfc:287` variable 'themeRenderer' has no component ref |
| 1 | `token → core/mura/client/api/oauth/oauthTokenBean.cfc` | 1 | high | declares getClient(), getToken() | `admin/core/views/cusers/edituser.cfm:749` variable 'token' has no component ref |
| 1 | `translator → core/mura/plugin/pluginStandardEventWrapper.cfc` | 1 | medium | declares handle() | `core/mura/Handler/standardEventsHandler.cfc:454` variable 'translator' has no component ref |
| 1 | `users → core/mura/iterator/queryIterator.cfc` | 1 | medium | declares next() | `admin/core/views/cwebservice/edit.cfm:72` variable 'users' has no component ref |
| 1 | `variables.bean → core/mura/bean/bean.cfc` | 25 | medium | declares getAllValues(); named like the receiver 'bean' (25 candidates) | `core/modules/v1/datacollection/dsp_response.cfm:88` variable 'variables.bean' has no component ref |
| 1 | `variables.collection → core/mura/cache/provider/cacheAdobe.cfc` | 2 | low | declares put() (2 candidates) | `core/mura/cache/cacheAbstract.cfc:196` variable 'variables.collection' has no component ref |
| 1 | `variables.content → core/mura/content/contentBean.cfc` | 2 | medium | declares setDisplayInterval(); named like the receiver 'content' (2 candidates) | `core/mura/content/contentDisplayIntervalBean.cfc:27` variable 'variables.content' has no component ref |
| 1 | `variables.crumb → core/mura/bean/beanExtendable.cfc` | 14 | low | declares getType() (14 candidates) | `core/modules/v1/nav/dsp_archive.cfm:88` variable 'variables.crumb' has no component ref |
| 1 | `variables.fbmanager → core/mura/formBuilder/formBuilderManager.cfc` | 1 | medium | declares renderNestedForm() | `core/modules/v1/formbuilder/fields/dsp_nested.cfm:95` variable 'variables.fbManager' has no component ref |
| 1 | `variables.iterator → core/mura/iterator/queryIterator.cfc` | 1 | medium | declares getRecordCount() | `core/modules/v1/related_content/index.cfm:87` variable 'variables.iterator' has no component ref |
| 1 | `variables.map` | 0 | none |  | `core/mura/cache/cacheAbstract.cfc:248` variable 'variables.map' has no component ref |
| 1 | `variables.map → core/mura/cache/cacheAbstract.cfc` | 3 | low | declares size() (3 candidates) | `core/mura/cache/cacheAbstract.cfc:201` variable 'variables.map' has no component ref |
| 1 | `variables.pluginconfigs[] → core/mura/plugin/pluginConfig.cfc` | 7 | low | declares getModuleID() (7 candidates) | `core/mura/plugin/pluginManager.cfc:951` variable 'variables.pluginConfigs[]' has no component ref |
| 1 | `variables.stats → core/mura/content/contentStatsBean.cfc` | 1 | medium | declares getLockType() | `admin/core/utilities/modal/toolbar.cfm:163` variable 'variables.stats' has no component ref |
| 1 | `zip` | 0 | none |  | `core/mura/Zip.cfc:793` variable 'zip' has no component ref |

<details><summary>Groups with several candidates</summary>

- `arguments.event → core/mura/event.cfc` — 304 finding(s), 13 candidate(s):
  - medium `core/mura/event.cfc` — declares getValue(); named like the receiver 'event'
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/servletEvent.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - low `core/mura/content/contentNavBean.cfc` — declares getValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue()
  - … 5 more
- `event → core/mura/event.cfc` — 183 finding(s), 7 candidate(s):
  - medium `core/mura/event.cfc` — declares getContentRenderer(); named like the receiver 'event'
  - low `core/mura/MasaScope.cfc` — declares getContentRenderer()
  - low `core/mura/queryParam.cfc` — declares getContentRenderer()
  - low `core/mura/servletEvent.cfc` — declares getContentRenderer()
  - low `core/mura/extend/extendData.cfc` — declares getContentRenderer()
  - low `core/mura/extend/extendSubType.cfc` — declares getContentRenderer()
  - low `core/mura/settings/settingsBean.cfc` — declares getContentRenderer()
- `mmrbf → core/mura/resourceBundle/resourceBundle.cfc` — 144 finding(s), 2 candidate(s):
  - low `core/mura/resourceBundle/resourceBundle.cfc` — declares getKeyValue()
  - low `core/mura/resourceBundle/resourceBundleFactory.cfc` — declares getKeyValue()
- `arguments.renderer → core/mura/content/contentRenderer.cfc` — 139 finding(s), 2 candidate(s):
  - low `core/mura/content/contentRenderer.cfc` — declares renderIcon()
  - low `core/mura/content/contentRendererUtility.cfc` — declares renderIcon()
- `variables.configbean → core/mura/configBean.cfc` — 72 finding(s), 2 candidate(s):
  - medium `core/mura/configBean.cfc` — declares getContext(), getServerPort(); named like the receiver 'configBean'
  - low `core/mura/settings/settingsBean.cfc` — declares getContext(), getServerPort()
- `arguments.bundle → core/mura/cfobject.cfc` — 37 finding(s), 13 candidate(s):
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/event.cfc` — declares getValue()
  - low `core/mura/servletEvent.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - low `core/mura/content/contentNavBean.cfc` — declares getValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue()
  - … 5 more
- `local.item → core/mura/cfobject.cfc` — 37 finding(s), 13 candidate(s):
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/event.cfc` — declares getValue()
  - low `core/mura/servletEvent.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - low `core/mura/content/contentNavBean.cfc` — declares getValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue()
  - … 5 more
- `entity → core/mura/bean/bean.cfc` — 32 finding(s), 2 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares getDynamic(), getTable(), getEntityName()
  - low `core/mura/bean/beanEntity.cfc` — declares getDynamic(), getTable(), getEntityName()
- `arguments.content → core/mura/content/contentBean.cfc` — 26 finding(s), 25 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentNavBean.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - … 17 more
- `request.userbean → core/mura/user/userBean.cfc` — 26 finding(s), 6 candidate(s):
  - medium `core/mura/user/userBean.cfc` — declares getCategoryID(); named like the receiver 'userBean'
  - low `core/mura/category/categoryBean.cfc` — declares getCategoryID()
  - low `core/mura/extend/extendSet.cfc` — declares getCategoryID()
  - low `core/mura/user/userFeedBean.cfc` — declares getCategoryID()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getCategoryID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getCategoryID()
- `feed → core/mura/content/feed/feedBean.cfc` — 25 finding(s), 2 candidate(s):
  - medium `core/mura/content/feed/feedBean.cfc` — declares getImageSize(), getImageWidth(), getImageHeight(), getdisplaylist(), getAvailabledisplaylist(); named like the receiver 'feed'
  - low `core/mura/content/contentBean.cfc` — declares getImageSize(), getImageWidth(), getImageHeight(), getdisplaylist(), getAvailabledisplaylist()
- `arguments.event → core/mura/content/contentBean.cfc` — 23 finding(s), 4 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getcontentID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getcontentID()
  - low `core/mura/content/rater/rateBean.cfc` — declares getcontentID()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getcontentID()
- `arguments.bundle → core/mura/settings/settingsBundle.cfc` — 22 finding(s), 2 candidate(s):
  - low `core/mura/settings/settingsBundle.cfc` — declares getValue(), unpackFiles(), renameFiles(), cleanUp()
  - low `core/mura/settings/settingsBundleBean.cfc` — declares getValue(), unpackFiles(), renameFiles(), cleanUp()
- `event → core/mura/content/contentBean.cfc` — 18 finding(s), 7 candidate(s):
  - medium `core/mura/content/contentBean.cfc` — declares getNextN(); named like the receiver 'ContentBean'
  - low `core/mura/utility.cfc` — declares getNextN()
  - low `core/mura/bean/beanFeed.cfc` — declares getNextN()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getNextN()
  - low `core/mura/iterator/queryIterator.cfc` — declares getNextN()
  - low `core/mura/settings/settingsBean.cfc` — declares getNextN()
  - low `core/mura/content/feed/feedBean.cfc` — declares getNextN()
- `request.servletevent → core/mura/servletEvent.cfc` — 18 finding(s), 13 candidate(s):
  - medium `core/mura/servletEvent.cfc` — declares getValue(); named like the receiver 'servletEvent'
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/event.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - low `core/mura/content/contentNavBean.cfc` — declares getValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue()
  - … 5 more
- `arguments.event → core/mura/MasaScope.cfc` — 17 finding(s), 3 candidate(s):
  - low `core/mura/MasaScope.cfc` — declares getValue(), setValue(), getContentBean()
  - low `core/mura/servletEvent.cfc` — declares getValue(), setValue(), getContentBean()
  - low `core/mura/content/contentNavBean.cfc` — declares getValue(), setValue(), getContentBean()
- `obj → core/mura/user/addressBean.cfc` — 14 finding(s), 5 candidate(s):
  - low `core/mura/user/addressBean.cfc` — declares setSiteID(), getPrimaryKey(), loadBy()
  - low `core/mura/user/userBean.cfc` — declares setSiteID(), getPrimaryKey(), loadBy()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares setSiteID(), getPrimaryKey(), loadBy()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares setSiteID(), getPrimaryKey(), loadBy()
  - low `core/mura/content/feed/feedBean.cfc` — declares setSiteID(), getPrimaryKey(), loadBy()
- `variables.formbean → core/mura/cfobject.cfc` — 13 finding(s), 13 candidate(s):
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/event.cfc` — declares getValue()
  - low `core/mura/servletEvent.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - low `core/mura/content/contentNavBean.cfc` — declares getValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue()
  - … 5 more
- `arguments.object → core/mura/bean/bean.cfc` — 9 finding(s), 2 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares getValidations()
  - low `core/mura/content/dataCollection/dataCollectionBean.cfc` — declares getValidations()
- `stats → core/mura/content/contentBean.cfc` — 8 finding(s), 2 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getMajorVersion(), getMinorVersion(), setMajorVersion(), setMinorVersion(), save()
  - low `core/mura/content/contentStatsBean.cfc` — declares getMajorVersion(), getMinorVersion(), setMajorVersion(), setMinorVersion(), save()
- `archivebean → core/mura/content/contentBean.cfc` — 7 finding(s), 2 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getIsNew(), getURL(), getFilename()
  - low `core/mura/content/file/fileBean.cfc` — declares getIsNew(), getURL(), getFilename()
- `arguments.data.bean → core/mura/bean/bean.cfc` — 7 finding(s), 11 candidate(s):
  - medium `core/mura/bean/bean.cfc` — declares getFeed(); named like the receiver 'bean'
  - low `core/mura/MasaScope.cfc` — declares getFeed()
  - low `core/mura/cfobject.cfc` — declares getFeed()
  - low `core/mura/bean/beanORM.cfc` — declares getFeed()
  - low `core/mura/iterator/queryIterator.cfc` — declares getFeed()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getFeed()
  - low `core/mura/content/changeset/changesetManager.cfc` — declares getFeed()
  - low `core/mura/content/feed/feedBean.cfc` — declares getFeed()
  - … 3 more
- `arguments.event → core/mura/content/contentRenderer.cfc` — 7 finding(s), 2 candidate(s):
  - low `core/mura/content/contentRenderer.cfc` — declares getCurrentURL()
  - low `core/mura/content/contentRendererUtility.cfc` — declares getCurrentURL()
- `arguments.feedbean → core/mura/content/feed/feedBean.cfc` — 7 finding(s), 2 candidate(s):
  - medium `core/mura/content/feed/feedBean.cfc` — declares getRestrictGroups(), getSiteID(), getIsNew(), getRestricted(); named like the receiver 'feedBean'
  - low `core/mura/content/contentBean.cfc` — declares getRestrictGroups(), getSiteID(), getIsNew(), getRestricted()
- `bundle → core/mura/cfobject.cfc` — 7 finding(s), 13 candidate(s):
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/event.cfc` — declares getValue()
  - low `core/mura/servletEvent.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - low `core/mura/content/contentNavBean.cfc` — declares getValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue()
  - … 5 more
- `formdatabean → core/mura/bean/bean.cfc` — 7 finding(s), 4 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares getValue(), getErrors()
  - low `core/mura/extend/extendAttribute.cfc` — declares getValue(), getErrors()
  - low `core/mura/extend/extendSet.cfc` — declares getValue(), getErrors()
  - low `core/mura/extend/extendSubType.cfc` — declares getValue(), getErrors()
- `arguments.contentbean → core/mura/bean/beanExtendable.cfc` — 6 finding(s), 6 candidate(s):
  - low `core/mura/bean/beanExtendable.cfc` — declares getSiteid(), getSubType(), getType()
  - low `core/mura/extend/extendData.cfc` — declares getSiteid(), getSubType(), getType()
  - low `core/mura/extend/extendObject.cfc` — declares getSiteid(), getSubType(), getType()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getSiteid(), getSubType(), getType()
  - low `core/mura/extend/extendSubType.cfc` — declares getSiteid(), getSubType(), getType()
  - low `core/mura/user/userBean.cfc` — declares getSiteid(), getSubType(), getType()
- `event → core/mura/content/contentNavBean.cfc` — 6 finding(s), 3 candidate(s):
  - low `core/mura/content/contentNavBean.cfc` — declares getContentBean(), setValue(), getValue()
  - low `core/mura/MasaScope.cfc` — declares getContentBean(), setValue(), getValue()
  - low `core/mura/servletEvent.cfc` — declares getContentBean(), setValue(), getValue()
- `variables.instance.content → core/mura/content/contentBean.cfc` — 6 finding(s), 15 candidate(s):
  - medium `core/mura/content/contentBean.cfc` — declares setIsNew(); named like the receiver 'content'
  - low `core/mura/content/contentCommentBean.cfc` — declares setIsNew()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares setIsNew()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares setIsNew()
  - low `core/mura/content/feed/feedBean.cfc` — declares setIsNew()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares setIsNew()
  - low `core/mura/bean/bean.cfc` — declares setIsNew()
  - low `core/mura/category/categoryBean.cfc` — declares setIsNew()
  - … 7 more
- `arguments.event → core/mura/bean/bean.cfc` — 5 finding(s), 4 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares exists()
  - low `core/mura/extend/extendAttribute.cfc` — declares exists()
  - low `core/mura/extend/extendSet.cfc` — declares exists()
  - low `core/mura/extend/extendSubType.cfc` — declares exists()
- `homebean → core/mura/content/contentBean.cfc` — 5 finding(s), 6 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getURL()
  - low `core/mura/content/contentCommentBean.cfc` — declares getURL()
  - low `core/mura/content/contentManager.cfc` — declares getURL()
  - low `core/mura/content/contentNavBean.cfc` — declares getURL()
  - low `core/mura/user/userRedirectBean.cfc` — declares getURL()
  - low `core/mura/content/file/fileBean.cfc` — declares getURL()
- `qs → core/mura/bean/beanFeed.cfc` — 5 finding(s), 3 candidate(s):
  - low `core/mura/bean/beanFeed.cfc` — declares addParam()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares addParam()
  - low `core/mura/content/feed/feedBean.cfc` — declares addParam()
- `rc.contentbean → core/mura/bean/beanExtendable.cfc` — 5 finding(s), 6 candidate(s):
  - low `core/mura/bean/beanExtendable.cfc` — declares getSubType(), getType()
  - low `core/mura/extend/extendData.cfc` — declares getSubType(), getType()
  - low `core/mura/extend/extendObject.cfc` — declares getSubType(), getType()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getSubType(), getType()
  - low `core/mura/extend/extendSubType.cfc` — declares getSubType(), getType()
  - low `core/mura/user/userBean.cfc` — declares getSubType(), getType()
- `rc.it → core/mura/bean/beanFeed.cfc` — 5 finding(s), 2 candidate(s):
  - low `core/mura/bean/beanFeed.cfc` — declares getPageIndex()
  - low `core/mura/iterator/queryIterator.cfc` — declares getPageIndex()
- `subitem → core/mura/bean/bean.cfc` — 5 finding(s), 2 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares getValue(), getIsNew(), setValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue(), getIsNew(), setValue()
- `arguments.bundle → core/mura/bean/bean.cfc` — 4 finding(s), 13 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares setValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares setValue()
  - low `core/mura/cfobject.cfc` — declares setValue()
  - low `core/mura/configBean.cfc` — declares setValue()
  - low `core/mura/event.cfc` — declares setValue()
  - low `core/mura/servletEvent.cfc` — declares setValue()
  - low `core/mura/content/contentNavBean.cfc` — declares setValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares setValue()
  - … 5 more
- `arguments.entity → core/mura/bean/bean.cfc` — 4 finding(s), 6 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares getAll()
  - low `core/mura/cache/cacheAbstract.cfc` — declares getAll()
  - low `core/mura/cache/cacheAdvanced.cfc` — declares getAll()
  - low `core/mura/user/sessionUserFacade.cfc` — declares getAll()
  - low `core/mura/cache/provider/cacheAdobe.cfc` — declares getAll()
  - low `core/mura/cache/provider/cacheLucee.cfc` — declares getAll()
- `arguments.event → core/mura/category/categoryBean.cfc` — 4 finding(s), 6 candidate(s):
  - low `core/mura/category/categoryBean.cfc` — declares getFilename()
  - low `core/mura/content/contentBean.cfc` — declares getFilename()
  - low `core/mura/content/contentCategoryAssignBean.cfc` — declares getFilename()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares getFilename()
  - low `core/mura/content/contentFilenameArchiveBean.cfc` — declares getFilename()
  - low `core/mura/content/file/fileBean.cfc` — declares getFilename()
- `arguments.renderer → core/mura/content/contentNavBean.cfc` — 4 finding(s), 13 candidate(s):
  - low `core/mura/content/contentNavBean.cfc` — declares getValue()
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/event.cfc` — declares getValue()
  - low `core/mura/servletEvent.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue()
  - … 5 more
- `attributes.pluginevent → core/mura/cfobject.cfc` — 4 finding(s), 13 candidate(s):
  - low `core/mura/cfobject.cfc` — declares setValue()
  - low `core/mura/configBean.cfc` — declares setValue()
  - low `core/mura/event.cfc` — declares setValue()
  - low `core/mura/servletEvent.cfc` — declares setValue()
  - low `core/mura/bean/bean.cfc` — declares setValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares setValue()
  - low `core/mura/content/contentNavBean.cfc` — declares setValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares setValue()
  - … 5 more
- `bean → core/mura/cfobject.cfc` — 4 finding(s), 2 candidate(s):
  - low `core/mura/cfobject.cfc` — declares invokeMethod()
  - low `core/mura/utility.cfc` — declares invokeMethod()
- `iterator → core/mura/bean/bean.cfc` — 4 finding(s), 9 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares getEntityName()
  - low `core/mura/bean/beanFeed.cfc` — declares getEntityName()
  - low `core/mura/bean/beanIterator.cfc` — declares getEntityName()
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares getEntityName()
  - low `core/mura/content/contentIterator.cfc` — declares getEntityName()
  - low `core/mura/user/addressIterator.cfc` — declares getEntityName()
  - low `core/mura/user/userBean.cfc` — declares getEntityName()
  - low `core/mura/user/userIterator.cfc` — declares getEntityName()
  - … 1 more
- `request.contentrenderer → core/mura/content/contentRenderer.cfc` — 4 finding(s), 4 candidate(s):
  - medium `core/mura/content/contentRenderer.cfc` — declares createHREF(); named like the receiver 'contentRenderer'
  - low `core/mura/MasaScope.cfc` — declares createHREF()
  - low `core/mura/content/contentRendererUtility.cfc` — declares createHREF()
  - low `core/mura/content/staticContentRenderer.cfc` — declares createHREF()
- `request.event → core/mura/event.cfc` — 4 finding(s), 7 candidate(s):
  - medium `core/mura/event.cfc` — declares getContentRenderer(); named like the receiver 'event'
  - low `core/mura/extend/extendData.cfc` — declares getContentRenderer()
  - low `core/mura/extend/extendSubType.cfc` — declares getContentRenderer()
  - low `core/mura/MasaScope.cfc` — declares getContentRenderer()
  - low `core/mura/queryParam.cfc` — declares getContentRenderer()
  - low `core/mura/servletEvent.cfc` — declares getContentRenderer()
  - low `core/mura/settings/settingsBean.cfc` — declares getContentRenderer()
- `ro → core/mura/content/contentCommentBean.cfc` — 4 finding(s), 10 candidate(s):
  - low `core/mura/content/contentCommentBean.cfc` — declares getPrimaryKey(), getValue()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getPrimaryKey(), getValue()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getPrimaryKey(), getValue()
  - low `core/mura/content/feed/feedBean.cfc` — declares getPrimaryKey(), getValue()
  - low `core/mura/bean/bean.cfc` — declares getPrimaryKey(), getValue()
  - low `core/mura/bean/beanORM.cfc` — declares getPrimaryKey(), getValue()
  - low `core/mura/category/categoryBean.cfc` — declares getPrimaryKey(), getValue()
  - low `core/mura/email/emailBean.cfc` — declares getPrimaryKey(), getValue()
  - … 2 more
- `sample → core/mura/content/contentCommentFeedBean.cfc` — 4 finding(s), 4 candidate(s):
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares getTable(), getPrimarykey()
  - low `core/mura/bean/bean.cfc` — declares getTable(), getPrimarykey()
  - low `core/mura/bean/beanFeed.cfc` — declares getTable(), getPrimarykey()
  - low `core/mura/category/categoryFeedBean.cfc` — declares getTable(), getPrimarykey()
- `subtype → core/mura/extend/extendRelatedContentSetBean.cfc` — 4 finding(s), 3 candidate(s):
  - low `core/mura/extend/extendRelatedContentSetBean.cfc` — declares getSubTypeID()
  - low `core/mura/extend/extendSet.cfc` — declares getSubTypeID()
  - low `core/mura/extend/extendSubType.cfc` — declares getSubTypeID()
- `variables.data.$ → core/mura/cfobject.cfc` — 4 finding(s), 2 candidate(s):
  - low `core/mura/cfobject.cfc` — declares getServiceFactory(), getBean()
  - low `core/mura/event.cfc` — declares getServiceFactory(), getBean()
- `application.classextensionmanager → admin/core/controllers/cextend.cfc` — 3 finding(s), 2 candidate(s):
  - low `admin/core/controllers/cextend.cfc` — declares saveAttributeSort()
  - low `core/mura/extend/extendManager.cfc` — declares saveAttributeSort()
- `arguments.credentials → core/mura/googleAuth.cfc` — 3 finding(s), 2 candidate(s):
  - low `core/mura/googleAuth.cfc` — declares getKey()
  - low `core/mura/resourceBundle/resourceBundleFactory.cfc` — declares getKey()
- `arguments.data → core/mura/configBean.cfc` — 3 finding(s), 25 candidate(s):
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - … 17 more
- `arguments.data → core/mura/user/sessionUserFacade.cfc` — 3 finding(s), 25 candidate(s):
  - low `core/mura/user/sessionUserFacade.cfc` — declares getAllValues()
  - low `core/mura/user/userBean.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - … 17 more
- `arguments.event → core/mura/json.cfc` — 3 finding(s), 20 candidate(s):
  - low `core/mura/json.cfc` — declares validate()
  - low `core/mura/queryParam.cfc` — declares validate()
  - low `core/mura/bean/bean.cfc` — declares validate()
  - low `core/mura/bean/beanORM.cfc` — declares validate()
  - low `core/mura/bean/beanValidator.cfc` — declares validate()
  - low `core/mura/content/contentBean.cfc` — declares validate()
  - low `core/mura/content/contentManager.cfc` — declares validate()
  - low `core/mura/extend/extendAttribute.cfc` — declares validate()
  - … 12 more
- `arguments.obj → core/mura/bean/bean.cfc` — 3 finding(s), 13 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares setValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares setValue()
  - low `core/mura/cfobject.cfc` — declares setValue()
  - low `core/mura/configBean.cfc` — declares setValue()
  - low `core/mura/event.cfc` — declares setValue()
  - low `core/mura/servletEvent.cfc` — declares setValue()
  - low `core/mura/content/contentNavBean.cfc` — declares setValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares setValue()
  - … 5 more
- `bundle → core/mura/settings/settingsBundle.cfc` — 3 finding(s), 2 candidate(s):
  - low `core/mura/settings/settingsBundle.cfc` — declares getBundle()
  - low `core/mura/settings/settingsBundleBean.cfc` — declares getBundle()
- `cache → core/mura/cache/cacheAbstract.cfc` — 3 finding(s), 4 candidate(s):
  - low `core/mura/cache/cacheAbstract.cfc` — declares purge()
  - low `core/mura/cache/cacheAdvanced.cfc` — declares purge()
  - low `core/mura/cache/provider/cacheAdobe.cfc` — declares purge()
  - low `core/mura/cache/provider/cacheLucee.cfc` — declares purge()
- `cat → core/mura/bean/beanEntity.cfc` — 3 finding(s), 5 candidate(s):
  - low `core/mura/bean/beanEntity.cfc` — declares getPath()
  - low `core/mura/category/categoryBean.cfc` — declares getPath()
  - low `core/mura/content/contentBean.cfc` — declares getPath()
  - low `core/mura/content/contentCategoryAssignBean.cfc` — declares getPath()
  - low `core/mura/content/contentCommentBean.cfc` — declares getPath()
- `crumb → core/mura/content/contentBean.cfc` — 3 finding(s), 2 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getType(), getContentID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getType(), getContentID()
- `email → core/mura/mailer.cfc` — 3 finding(s), 2 candidate(s):
  - low `core/mura/mailer.cfc` — declares sendText()
  - low `core/mura/mailerLimited.cfc` — declares sendText()
- `entity → core/mura/bean/beanEntity.cfc` — 3 finding(s), 3 candidate(s):
  - low `core/mura/bean/beanEntity.cfc` — declares getName(), getDisplayName()
  - low `core/mura/extend/extendRelatedContentSetBean.cfc` — declares getName(), getDisplayName()
  - low `core/mura/content/feed/feedBean.cfc` — declares getName(), getDisplayName()
- `entity → core/mura/bean/beanExtendable.cfc` — 3 finding(s), 26 candidate(s):
  - low `core/mura/bean/beanExtendable.cfc` — declares getSiteID()
  - low `core/mura/bean/beanFeed.cfc` — declares getSiteID()
  - low `core/mura/category/categoryFeedBean.cfc` — declares getSiteID()
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares getSiteID()
  - low `core/mura/email/emailBean.cfc` — declares getSiteID()
  - low `core/mura/extend/extendAttribute.cfc` — declares getSiteID()
  - low `core/mura/extend/extendData.cfc` — declares getSiteID()
  - low `core/mura/extend/extendObject.cfc` — declares getSiteID()
  - … 18 more
- `item → core/mura/bean/bean.cfc` — 3 finding(s), 5 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares getEntityName(), getPrimaryKey()
  - low `core/mura/bean/beanFeed.cfc` — declares getEntityName(), getPrimaryKey()
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares getEntityName(), getPrimaryKey()
  - low `core/mura/user/userBean.cfc` — declares getEntityName(), getPrimaryKey()
  - low `core/mura/content/feed/feedBean.cfc` — declares getEntityName(), getPrimaryKey()
- `newfeedbean → core/mura/content/contentBean.cfc` — 3 finding(s), 4 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getIsNew(), setContentID(), save()
  - low `core/mura/content/dataCollection/dataCollectionBean.cfc` — declares getIsNew(), setContentID(), save()
  - low `core/mura/content/feed/feedBean.cfc` — declares getIsNew(), setContentID(), save()
  - low `core/mura/content/rater/rateBean.cfc` — declares getIsNew(), setContentID(), save()
- `obj → core/mura/content/contentCommentFeedBean.cfc` — 3 finding(s), 5 candidate(s):
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares validate(), getEntityName()
  - low `core/mura/content/feed/feedBean.cfc` — declares validate(), getEntityName()
  - low `core/mura/bean/bean.cfc` — declares validate(), getEntityName()
  - low `core/mura/bean/beanFeed.cfc` — declares validate(), getEntityName()
  - low `core/mura/user/userBean.cfc` — declares validate(), getEntityName()
- `obj → core/mura/user/userBean.cfc` — 3 finding(s), 5 candidate(s):
  - low `core/mura/user/userBean.cfc` — declares validate(), getEntityName()
  - low `core/mura/bean/bean.cfc` — declares validate(), getEntityName()
  - low `core/mura/bean/beanFeed.cfc` — declares validate(), getEntityName()
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares validate(), getEntityName()
  - low `core/mura/content/feed/feedBean.cfc` — declares validate(), getEntityName()
- `parentbean → core/mura/bean/beanExtendable.cfc` — 3 finding(s), 6 candidate(s):
  - low `core/mura/bean/beanExtendable.cfc` — declares getSiteID(), getSubType(), getType()
  - low `core/mura/extend/extendData.cfc` — declares getSiteID(), getSubType(), getType()
  - low `core/mura/extend/extendObject.cfc` — declares getSiteID(), getSubType(), getType()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getSiteID(), getSubType(), getType()
  - low `core/mura/extend/extendSubType.cfc` — declares getSiteID(), getSubType(), getType()
  - low `core/mura/user/userBean.cfc` — declares getSiteID(), getSubType(), getType()
- `pluginevent → core/mura/content/contentNavBean.cfc` — 3 finding(s), 13 candidate(s):
  - low `core/mura/content/contentNavBean.cfc` — declares getValue(), setValue()
  - low `core/mura/cfobject.cfc` — declares getValue(), setValue()
  - low `core/mura/configBean.cfc` — declares getValue(), setValue()
  - low `core/mura/event.cfc` — declares getValue(), setValue()
  - low `core/mura/servletEvent.cfc` — declares getValue(), setValue()
  - low `core/mura/bean/bean.cfc` — declares getValue(), setValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue(), setValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue(), setValue()
  - … 5 more
- `variables.contentgateway → core/mura/dashboard/dashboardManager.cfc` — 3 finding(s), 2 candidate(s):
  - low `core/mura/dashboard/dashboardManager.cfc` — declares getRecentUpdates()
  - low `core/mura/content/contentGatewayAdobe.cfc` — declares getRecentUpdates()
- `variables.crumb → core/mura/content/contentBean.cfc` — 3 finding(s), 2 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getType(), getContentID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getType(), getContentID()
- `variables.event → core/mura/event.cfc` — 3 finding(s), 13 candidate(s):
  - medium `core/mura/event.cfc` — declares setValue(); named like the receiver 'event'
  - low `core/mura/cfobject.cfc` — declares setValue()
  - low `core/mura/configBean.cfc` — declares setValue()
  - low `core/mura/servletEvent.cfc` — declares setValue()
  - low `core/mura/bean/bean.cfc` — declares setValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares setValue()
  - low `core/mura/content/contentNavBean.cfc` — declares setValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares setValue()
  - … 5 more
- `address → core/mura/content/contentBean.cfc` — 2 finding(s), 6 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares save(), getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares save(), getAllValues()
  - low `core/mura/extend/extendAttribute.cfc` — declares save(), getAllValues()
  - low `core/mura/extend/extendSet.cfc` — declares save(), getAllValues()
  - low `core/mura/extend/extendSubType.cfc` — declares save(), getAllValues()
  - low `core/mura/user/userBean.cfc` — declares save(), getAllValues()
- `ap → core/mura/content/contentBean.cfc` — 2 finding(s), 15 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getIsNew()
  - low `core/mura/content/contentCommentBean.cfc` — declares getIsNew()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getIsNew()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getIsNew()
  - low `core/mura/content/feed/feedBean.cfc` — declares getIsNew()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getIsNew()
  - low `core/mura/bean/bean.cfc` — declares getIsNew()
  - low `core/mura/category/categoryBean.cfc` — declares getIsNew()
  - … 7 more
- `application.confibean → core/mura/configBean.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/configBean.cfc` — declares getFileDir()
  - low `core/mura/settings/settingsBean.cfc` — declares getFileDir()
- `approvalrequest → core/mura/content/approval/approvalRequestBean.cfc` — 2 finding(s), 3 candidate(s):
  - medium `core/mura/content/approval/approvalRequestBean.cfc` — declares getIsNew(), getStatus(); named like the receiver 'approvalRequest'
  - low `core/mura/content/contentBean.cfc` — declares getIsNew(), getStatus()
  - low `core/mura/email/emailBean.cfc` — declares getIsNew(), getStatus()
- `archived → core/mura/content/contentBean.cfc` — 2 finding(s), 4 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getIsNew(), getContentID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getIsNew(), getContentID()
  - low `core/mura/content/rater/rateBean.cfc` — declares getIsNew(), getContentID()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getIsNew(), getContentID()
- `arguments.address → core/mura/user/userBean.cfc` — 2 finding(s), 3 candidate(s):
  - low `core/mura/user/userBean.cfc` — declares setSiteID(), setUserID()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares setSiteID(), setUserID()
  - low `core/mura/content/rater/rateBean.cfc` — declares setSiteID(), setUserID()
- `arguments.applicationscope.pluginmanager → core/mura/plugin/pluginManager.cfc` — 2 finding(s), 2 candidate(s):
  - medium `core/mura/plugin/pluginManager.cfc` — declares announceEvent(); named like the receiver 'pluginManager'
  - low `core/mura/MasaScope.cfc` — declares announceEvent()
- `arguments.cache → core/mura/cache/cacheAbstract.cfc` — 2 finding(s), 4 candidate(s):
  - low `core/mura/cache/cacheAbstract.cfc` — declares purge()
  - low `core/mura/cache/cacheAdvanced.cfc` — declares purge()
  - low `core/mura/cache/provider/cacheAdobe.cfc` — declares purge()
  - low `core/mura/cache/provider/cacheLucee.cfc` — declares purge()
- `arguments.child → core/mura/formBuilder/datasetBean.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/formBuilder/datasetBean.cfc` — declares setSiteID(), setParentID()
  - low `core/mura/content/feed/feedBean.cfc` — declares setSiteID(), setParentID()
- `arguments.displayinterval → core/mura/content/contentBean.cfc` — 2 finding(s), 25 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentNavBean.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - … 17 more
- `arguments.iterator → core/mura/bean/beanFeed.cfc` — 2 finding(s), 6 candidate(s):
  - low `core/mura/bean/beanFeed.cfc` — declares setNextN()
  - low `core/mura/content/contentBean.cfc` — declares setNextN()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares setNextN()
  - low `core/mura/iterator/queryIterator.cfc` — declares setNextN()
  - low `core/mura/settings/settingsBean.cfc` — declares setNextN()
  - low `core/mura/content/feed/feedBean.cfc` — declares setNextN()
- `arguments.rc.attributebean → admin/core/controllers/cchain.cfc` — 2 finding(s), 45 candidate(s):
  - low `admin/core/controllers/cchain.cfc` — declares save()
  - low `admin/core/controllers/cchangesets.cfc` — declares save()
  - low `admin/core/controllers/cwebservice.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - … 37 more
- `arguments.rc.extendsetbean → core/mura/extend/extendSet.cfc` — 2 finding(s), 45 candidate(s):
  - medium `core/mura/extend/extendSet.cfc` — declares save(); named like the receiver 'extendSetBean'
  - low `admin/core/controllers/cchain.cfc` — declares save()
  - low `admin/core/controllers/cchangesets.cfc` — declares save()
  - low `admin/core/controllers/cwebservice.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - … 37 more
- `arguments.rc.subtypebean → admin/core/controllers/cchain.cfc` — 2 finding(s), 45 candidate(s):
  - low `admin/core/controllers/cchain.cfc` — declares save()
  - low `admin/core/controllers/cchangesets.cfc` — declares save()
  - low `admin/core/controllers/cwebservice.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - … 37 more
- `attributes.murascope → core/mura/MasaScope.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/MasaScope.cfc` — declares renderCSRFTokens()
  - low `core/mura/user/sessionUserFacade.cfc` — declares renderCSRFTokens()
- `attributes.userbean → core/mura/user/userBean.cfc` — 2 finding(s), 6 candidate(s):
  - medium `core/mura/user/userBean.cfc` — declares getCategoryID(); named like the receiver 'userBean'
  - low `core/mura/category/categoryBean.cfc` — declares getCategoryID()
  - low `core/mura/extend/extendSet.cfc` — declares getCategoryID()
  - low `core/mura/user/userFeedBean.cfc` — declares getCategoryID()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getCategoryID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getCategoryID()
- `atts[i] → core/mura/extend/extendAttribute.cfc` — 2 finding(s), 25 candidate(s):
  - low `core/mura/extend/extendAttribute.cfc` — declares getAllValues()
  - low `core/mura/extend/extendData.cfc` — declares getAllValues()
  - low `core/mura/extend/extendSet.cfc` — declares getAllValues()
  - low `core/mura/extend/extendSubType.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - … 17 more
- `bean → core/mura/bean/beanORM.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/bean/beanORM.cfc` — declares toBundle()
  - low `core/mura/bean/beanORMVersioned.cfc` — declares toBundle()
- `cat → core/mura/category/categoryBean.cfc` — 2 finding(s), 4 candidate(s):
  - low `core/mura/category/categoryBean.cfc` — declares getName(), getCategoryID()
  - low `core/mura/extend/extendSet.cfc` — declares getName(), getCategoryID()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getName(), getCategoryID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getName(), getCategoryID()
- `content → core/mura/client/api/soap/v1/content.cfc` — 2 finding(s), 4 candidate(s):
  - low `core/mura/client/api/soap/v1/content.cfc` — declares deleteVersion(); named like the receiver 'content'
  - low `core/mura/content/contentBean.cfc` — declares deleteVersion(); named like the receiver 'content'
  - low `core/mura/content/file/fileDAO.cfc` — declares deleteVersion()
  - low `core/mura/content/file/fileManager.cfc` — declares deleteVersion()
- `current → core/mura/bean/beanFeed.cfc` — 2 finding(s), 8 candidate(s):
  - low `core/mura/bean/beanFeed.cfc` — declares setSortBy(), setSortDirection()
  - low `core/mura/category/categoryBean.cfc` — declares setSortBy(), setSortDirection()
  - low `core/mura/category/categoryFeedBean.cfc` — declares setSortBy(), setSortDirection()
  - low `core/mura/content/contentBean.cfc` — declares setSortBy(), setSortDirection()
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares setSortBy(), setSortDirection()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares setSortBy(), setSortDirection()
  - low `core/mura/user/userFeedBean.cfc` — declares setSortBy(), setSortDirection()
  - low `core/mura/content/feed/feedBean.cfc` — declares setSortBy(), setSortDirection()
- `databean → core/mura/bean/beanORM.cfc` — 2 finding(s), 8 candidate(s):
  - low `core/mura/bean/beanORM.cfc` — declares getPrimaryKey(), loadby()
  - low `core/mura/category/categoryBean.cfc` — declares getPrimaryKey(), loadby()
  - low `core/mura/content/contentCommentBean.cfc` — declares getPrimaryKey(), loadby()
  - low `core/mura/user/addressBean.cfc` — declares getPrimaryKey(), loadby()
  - low `core/mura/user/userBean.cfc` — declares getPrimaryKey(), loadby()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getPrimaryKey(), loadby()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getPrimaryKey(), loadby()
  - low `core/mura/content/feed/feedBean.cfc` — declares getPrimaryKey(), loadby()
- `event → core/mura/configBean.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/configBean.cfc` — declares getAssetPath()
  - low `core/mura/settings/settingsBean.cfc` — declares getAssetPath()
- `filemetadata → core/mura/MasaScope.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/MasaScope.cfc` — declares generateCSRFTokens(), renderCSRFTokens()
  - low `core/mura/user/sessionUserFacade.cfc` — declares generateCSRFTokens(), renderCSRFTokens()
- `item → core/mura/cfobject.cfc` — 2 finding(s), 13 candidate(s):
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/event.cfc` — declares getValue()
  - low `core/mura/servletEvent.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - low `core/mura/content/contentNavBean.cfc` — declares getValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue()
  - … 5 more
- `keys → core/mura/configBean.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/configBean.cfc` — declares getMode()
  - low `core/mura/publisherKeys.cfc` — declares getMode()
- `local.stats → admin/core/controllers/cchain.cfc` — 2 finding(s), 45 candidate(s):
  - low `admin/core/controllers/cchain.cfc` — declares save()
  - low `admin/core/controllers/cchangesets.cfc` — declares save()
  - low `admin/core/controllers/cwebservice.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - … 37 more
- `localobject → core/mura/bean/beanORM.cfc` — 2 finding(s), 8 candidate(s):
  - low `core/mura/bean/beanORM.cfc` — declares getPrimaryKey(), loadBy()
  - low `core/mura/category/categoryBean.cfc` — declares getPrimaryKey(), loadBy()
  - low `core/mura/content/contentCommentBean.cfc` — declares getPrimaryKey(), loadBy()
  - low `core/mura/user/addressBean.cfc` — declares getPrimaryKey(), loadBy()
  - low `core/mura/user/userBean.cfc` — declares getPrimaryKey(), loadBy()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getPrimaryKey(), loadBy()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getPrimaryKey(), loadBy()
  - low `core/mura/content/feed/feedBean.cfc` — declares getPrimaryKey(), loadBy()
- `newattribute → core/mura/content/contentBean.cfc` — 2 finding(s), 5 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares setSiteID(), setOrderno()
  - low `core/mura/extend/extendAttribute.cfc` — declares setSiteID(), setOrderno()
  - low `core/mura/extend/extendRelatedContentSetBean.cfc` — declares setSiteID(), setOrderno()
  - low `core/mura/extend/extendSet.cfc` — declares setSiteID(), setOrderno()
  - low `core/mura/settings/settingsBean.cfc` — declares setSiteID(), setOrderno()
- `obj → core/mura/bean/beanEntity.cfc` — 2 finding(s), 45 candidate(s):
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - … 37 more
- `obj → core/mura/category/categoryBean.cfc` — 2 finding(s), 45 candidate(s):
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - … 37 more
- `obj → core/mura/content/contentBean.cfc` — 2 finding(s), 45 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - low `core/mura/content/contentManager.cfc` — declares save()
  - low `core/mura/content/contentStatsBean.cfc` — declares save()
  - low `core/mura/content/approval/approvalChainBean.cfc` — declares save()
  - low `core/mura/content/approval/approvalRequestBean.cfc` — declares save()
  - … 37 more
- `object → core/mura/cfobject.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/cfobject.cfc` — declares invokeMethod()
  - low `core/mura/utility.cfc` — declares invokeMethod()
- `rbfactory → core/modules/v1/filebrowser/model/beans/filebrowser.cfc` — 2 finding(s), 3 candidate(s):
  - low `core/modules/v1/filebrowser/model/beans/filebrowser.cfc` — declares getResourceBundle()
  - low `core/mura/resourceBundle/resourceBundle.cfc` — declares getResourceBundle()
  - low `core/mura/resourceBundle/resourceBundleFactory.cfc` — declares getResourceBundle()
- `rc.contentbean → admin/core/controllers/cchain.cfc` — 2 finding(s), 45 candidate(s):
  - low `admin/core/controllers/cchain.cfc` — declares save()
  - low `admin/core/controllers/cchangesets.cfc` — declares save()
  - low `admin/core/controllers/cwebservice.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - … 37 more
- `rc.parentbean → core/mura/bean/beanExtendable.cfc` — 2 finding(s), 14 candidate(s):
  - low `core/mura/bean/beanExtendable.cfc` — declares getType()
  - low `core/mura/content/contentBean.cfc` — declares getType()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares getType()
  - low `core/mura/extend/extendAttribute.cfc` — declares getType()
  - low `core/mura/extend/extendData.cfc` — declares getType()
  - low `core/mura/extend/extendObject.cfc` — declares getType()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getType()
  - low `core/mura/extend/extendSubType.cfc` — declares getType()
  - … 6 more
- `relatedcontentbean → core/mura/content/contentBean.cfc` — 2 finding(s), 4 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getIsNew(), getContentID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getIsNew(), getContentID()
  - low `core/mura/content/rater/rateBean.cfc` — declares getIsNew(), getContentID()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getIsNew(), getContentID()
- `request.muraglobalevent → core/mura/configBean.cfc` — 2 finding(s), 25 candidate(s):
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - … 17 more
- `session.mura.editbean → core/mura/bean/beanEntity.cfc` — 2 finding(s), 12 candidate(s):
  - low `core/mura/bean/beanEntity.cfc` — declares getLastUpdate(), setLastUpdate()
  - low `core/mura/bean/beanORMHistorical.cfc` — declares getLastUpdate(), setLastUpdate()
  - low `core/mura/category/categoryBean.cfc` — declares getLastUpdate(), setLastUpdate()
  - low `core/mura/content/contentBean.cfc` — declares getLastUpdate(), setLastUpdate()
  - low `core/mura/email/emailBean.cfc` — declares getLastUpdate(), setLastUpdate()
  - low `core/mura/formBuilder/entityBean.cfc` — declares getLastUpdate(), setLastUpdate()
  - low `core/mura/mailinglist/mailinglistBean.cfc` — declares getLastUpdate(), setLastUpdate()
  - low `core/mura/user/userBean.cfc` — declares getLastUpdate(), setLastUpdate()
  - … 4 more
- `sites[] → core/mura/content/contentRenderer.cfc` — 2 finding(s), 3 candidate(s):
  - low `core/mura/content/contentRenderer.cfc` — declares registerDisplayObject()
  - low `core/mura/settings/settingsBean.cfc` — declares registerDisplayObject()
  - low `core/mura/client/api/json/v1/jsonApiUtility.cfc` — declares registerDisplayObject()
- `sites[] → core/mura/settings/settingsBean.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/settings/settingsBean.cfc` — declares getAccessControlOriginDomainList()
  - low `core/mura/settings/settingsManager.cfc` — declares getAccessControlOriginDomainList()
- `variables.contentgateway → core/mura/content/contentBean.cfc` — 2 finding(s), 6 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getKids()
  - low `core/mura/content/contentCategoryAssignBean.cfc` — declares getKids()
  - low `core/mura/content/contentCommentBean.cfc` — declares getKids()
  - low `core/mura/content/contentGatewayAdobe.cfc` — declares getKids()
  - low `core/mura/category/categoryBean.cfc` — declares getKids()
  - low `core/mura/client/api/soap/v1/content.cfc` — declares getKids()
- `variables.data.$ → core/mura/bean/beanFactory.cfc` — 2 finding(s), 3 candidate(s):
  - low `core/mura/bean/beanFactory.cfc` — declares containsBean()
  - low `core/mura/bean/ioc.cfc` — declares containsBean()
  - low `core/mura/plugin/pluginApplication.cfc` — declares containsBean()
- `variables.parent → core/mura/bean/ioc.cfc` — 2 finding(s), 20 candidate(s):
  - low `core/mura/bean/ioc.cfc` — declares getBean()
  - low `core/mura/MasaScope.cfc` — declares getBean()
  - low `core/mura/cfobject.cfc` — declares getBean()
  - low `core/mura/event.cfc` — declares getBean()
  - low `core/mura/servletEvent.cfc` — declares getBean()
  - low `core/mura/category/categoryFeedBean.cfc` — declares getBean()
  - low `core/mura/content/contentManager.cfc` — declares getBean()
  - low `core/mura/email/emailManager.cfc` — declares getBean()
  - … 12 more
- `variables.parentfactory → core/mura/resourceBundle/resourceBundle.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/resourceBundle/resourceBundle.cfc` — declares getKeyValue()
  - low `core/mura/resourceBundle/resourceBundleFactory.cfc` — declares getKeyValue()
- `variables.rbfactory → core/modules/v1/filebrowser/model/beans/filebrowser.cfc` — 2 finding(s), 3 candidate(s):
  - low `core/modules/v1/filebrowser/model/beans/filebrowser.cfc` — declares getResourceBundle()
  - low `core/mura/resourceBundle/resourceBundle.cfc` — declares getResourceBundle()
  - low `core/mura/resourceBundle/resourceBundleFactory.cfc` — declares getResourceBundle()
- `variables.sectionbean → core/mura/configBean.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/configBean.cfc` — declares getTitle()
  - low `core/mura/content/contentBean.cfc` — declares getTitle()
- `variables.userbean → core/mura/user/userBean.cfc` — 2 finding(s), 12 candidate(s):
  - medium `core/mura/user/userBean.cfc` — declares getIsNew(), setSiteID(); named like the receiver 'userBean'
  - low `core/mura/user/addressBean.cfc` — declares getIsNew(), setSiteID()
  - low `core/mura/content/contentBean.cfc` — declares getIsNew(), setSiteID()
  - low `core/mura/extend/extendAttribute.cfc` — declares getIsNew(), setSiteID()
  - low `core/mura/extend/extendSet.cfc` — declares getIsNew(), setSiteID()
  - low `core/mura/extend/extendSubType.cfc` — declares getIsNew(), setSiteID()
  - low `core/mura/mailinglist/mailinglistBean.cfc` — declares getIsNew(), setSiteID()
  - low `core/mura/settings/settingsImageSizeBean.cfc` — declares getIsNew(), setSiteID()
  - … 4 more
- `action → core/mura/user/userBean.cfc` — 1 finding(s), 2 candidate(s):
  - medium `core/mura/user/userBean.cfc` — declares getFullName(); named like the receiver 'User'
  - low `core/mura/user/sessionUserFacade.cfc` — declares getFullName()
- `activebean → core/mura/content/contentBean.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getURLtitle()
  - low `core/mura/category/categoryBean.cfc` — declares getURLtitle()
- `address → core/mura/user/addressBean.cfc` — 1 finding(s), 45 candidate(s):
  - medium `core/mura/user/addressBean.cfc` — declares save(); named like the receiver 'address'
  - low `core/mura/user/userBean.cfc` — declares save()
  - low `core/mura/user/userManager.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - … 37 more
- `admingroup → core/mura/content/contentCommentBean.cfc` — 1 finding(s), 4 candidate(s):
  - low `core/mura/content/contentCommentBean.cfc` — declares getUserID()
  - low `core/mura/user/userBean.cfc` — declares getUserID()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getUserID()
  - low `core/mura/content/rater/rateBean.cfc` — declares getUserID()
- `application.classextensionmanager → core/mura/extend/extendRelatedContentSetBean.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/extend/extendRelatedContentSetBean.cfc` — declares getAvailableSubTypes()
  - low `core/mura/extend/extendSubType.cfc` — declares getAvailableSubTypes()
- `application.classextensionmanager → core/mura/extend/extendSet.cfc` — 1 finding(s), 2 candidate(s):
  - medium `core/mura/extend/extendSet.cfc` — declares getattributeBean(); named like the receiver 'ExtendSetBean'
  - low `core/mura/plugin/pluginManager.cfc` — declares getattributeBean()
- `arguments.changesetbean → core/mura/content/changeset/changesetBean.cfc` — 1 finding(s), 2 candidate(s):
  - medium `core/mura/content/changeset/changesetBean.cfc` — declares getChangesetID(); named like the receiver 'changesetBean'
  - low `core/mura/content/contentBean.cfc` — declares getChangesetID()
- `arguments.contentbean → core/mura/content/contentCommentFeedBean.cfc` — 1 finding(s), 26 candidate(s):
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares getSiteID()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getSiteID()
  - low `core/mura/content/dataCollection/dataCollectionBean.cfc` — declares getSiteID()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getSiteID()
  - low `core/mura/content/rater/rateBean.cfc` — declares getSiteID()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getSiteID()
  - low `core/mura/bean/beanExtendable.cfc` — declares getSiteID()
  - low `core/mura/bean/beanFeed.cfc` — declares getSiteID()
  - … 18 more
- `arguments.contentbean → core/mura/formBuilder/datasetBean.cfc` — 1 finding(s), 26 candidate(s):
  - low `core/mura/formBuilder/datasetBean.cfc` — declares getSiteID()
  - low `core/mura/formBuilder/formBean.cfc` — declares getSiteID()
  - low `core/mura/bean/beanExtendable.cfc` — declares getSiteID()
  - low `core/mura/bean/beanFeed.cfc` — declares getSiteID()
  - low `core/mura/category/categoryFeedBean.cfc` — declares getSiteID()
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares getSiteID()
  - low `core/mura/email/emailBean.cfc` — declares getSiteID()
  - low `core/mura/extend/extendAttribute.cfc` — declares getSiteID()
  - … 18 more
- `arguments.data → core/mura/content/contentBean.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentNavBean.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - … 17 more
- `arguments.data → core/mura/settings/settingsBundle.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/settings/settingsBundle.cfc` — declares getAllValues()
  - low `core/mura/settings/settingsBundleBean.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - … 17 more
- `arguments.data.bean → core/mura/content/feed/feedBean.cfc` — 1 finding(s), 15 candidate(s):
  - medium `core/mura/content/feed/feedBean.cfc` — declares getQuery(); named like the receiver 'Feed'
  - low `core/mura/bean/beanFeed.cfc` — declares getQuery()
  - low `core/mura/content/contentCommentBean.cfc` — declares getQuery()
  - low `core/mura/content/contentStatsBean.cfc` — declares getQuery()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getQuery()
  - low `core/mura/iterator/queryIterator.cfc` — declares getQuery()
  - low `core/mura/plugin/pluginDisplayObjectBean.cfc` — declares getQuery()
  - low `core/mura/plugin/pluginScriptBean.cfc` — declares getQuery()
  - … 7 more
- `arguments.iterator → core/mura/bean/bean.cfc` — 1 finding(s), 9 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares getEntityName()
  - low `core/mura/bean/beanFeed.cfc` — declares getEntityName()
  - low `core/mura/bean/beanIterator.cfc` — declares getEntityName()
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares getEntityName()
  - low `core/mura/content/contentIterator.cfc` — declares getEntityName()
  - low `core/mura/user/addressIterator.cfc` — declares getEntityName()
  - low `core/mura/user/userBean.cfc` — declares getEntityName()
  - low `core/mura/user/userIterator.cfc` — declares getEntityName()
  - … 1 more
- `arguments.keyfactory → core/mura/configBean.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/configBean.cfc` — declares getMode()
  - low `core/mura/publisherKeys.cfc` — declares getMode()
- `arguments.loginobject → core/mura/login/loginManager.cfc` — 1 finding(s), 6 candidate(s):
  - low `core/mura/login/loginManager.cfc` — declares login()
  - low `core/mura/user/userBean.cfc` — declares login()
  - low `core/mura/user/userUtility.cfc` — declares login()
  - low `core/mura/client/api/json/v1/jsonApiUtility.cfc` — declares login()
  - low `core/mura/client/api/soap/v1/muraProxy.cfc` — declares login()
  - low `admin/core/controllers/clogin.cfc` — declares login()
- `arguments.object → core/mura/bean/beanExtendable.cfc` — 1 finding(s), 26 candidate(s):
  - low `core/mura/bean/beanExtendable.cfc` — declares getSiteID()
  - low `core/mura/bean/beanFeed.cfc` — declares getSiteID()
  - low `core/mura/category/categoryFeedBean.cfc` — declares getSiteID()
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares getSiteID()
  - low `core/mura/email/emailBean.cfc` — declares getSiteID()
  - low `core/mura/extend/extendAttribute.cfc` — declares getSiteID()
  - low `core/mura/extend/extendData.cfc` — declares getSiteID()
  - low `core/mura/extend/extendObject.cfc` — declares getSiteID()
  - … 18 more
- `arguments.parentbean → core/mura/content/contentBean.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getTitle()
  - low `core/mura/configBean.cfc` — declares getTitle()
- `arguments.qs → core/mura/bean/beanFeed.cfc` — 1 finding(s), 3 candidate(s):
  - low `core/mura/bean/beanFeed.cfc` — declares addParam()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares addParam()
  - low `core/mura/content/feed/feedBean.cfc` — declares addParam()
- `arguments.rc.contentbean → core/mura/bean/bean.cfc` — 1 finding(s), 4 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares getErrors()
  - low `core/mura/extend/extendAttribute.cfc` — declares getErrors()
  - low `core/mura/extend/extendSet.cfc` — declares getErrors()
  - low `core/mura/extend/extendSubType.cfc` — declares getErrors()
- `arguments.rc.rcsbean → admin/core/controllers/cchain.cfc` — 1 finding(s), 45 candidate(s):
  - low `admin/core/controllers/cchain.cfc` — declares save()
  - low `admin/core/controllers/cchangesets.cfc` — declares save()
  - low `admin/core/controllers/cwebservice.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - … 37 more
- `arguments.renderer → core/mura/content/contentBean.cfc` — 1 finding(s), 15 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getIsNew()
  - low `core/mura/content/contentCommentBean.cfc` — declares getIsNew()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getIsNew()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getIsNew()
  - low `core/mura/content/feed/feedBean.cfc` — declares getIsNew()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getIsNew()
  - low `core/mura/bean/bean.cfc` — declares getIsNew()
  - low `core/mura/category/categoryBean.cfc` — declares getIsNew()
  - … 7 more
- `arguments.renderer → core/mura/event.cfc` — 1 finding(s), 13 candidate(s):
  - medium `core/mura/event.cfc` — declares getValue(); named like the receiver 'Event'
  - low `core/mura/content/contentNavBean.cfc` — declares getValue()
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/servletEvent.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue()
  - … 5 more
- `attributes.cachefactory → core/mura/cache/cacheAbstract.cfc` — 1 finding(s), 4 candidate(s):
  - low `core/mura/cache/cacheAbstract.cfc` — declares has(), purge()
  - low `core/mura/cache/cacheAdvanced.cfc` — declares has(), purge()
  - low `core/mura/cache/provider/cacheAdobe.cfc` — declares has(), purge()
  - low `core/mura/cache/provider/cacheLucee.cfc` — declares has(), purge()
- `attributes.feedbean → core/mura/content/feed/feedBean.cfc` — 1 finding(s), 6 candidate(s):
  - medium `core/mura/content/feed/feedBean.cfc` — declares getCategoryID(); named like the receiver 'feedBean'
  - low `core/mura/category/categoryBean.cfc` — declares getCategoryID()
  - low `core/mura/extend/extendSet.cfc` — declares getCategoryID()
  - low `core/mura/user/userBean.cfc` — declares getCategoryID()
  - low `core/mura/user/userFeedBean.cfc` — declares getCategoryID()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getCategoryID()
- `attributes.rc.$ → admin/Application.cfc` — 1 finding(s), 2 candidate(s):
  - low `admin/Application.cfc` — declares rbKey()
  - low `core/mura/MasaScope.cfc` — declares rbKey()
- `calendar → core/mura/category/categoryBean.cfc` — 1 finding(s), 6 candidate(s):
  - low `core/mura/category/categoryBean.cfc` — declares getFilename()
  - low `core/mura/content/contentBean.cfc` — declares getFilename()
  - low `core/mura/content/contentCategoryAssignBean.cfc` — declares getFilename()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares getFilename()
  - low `core/mura/content/contentFilenameArchiveBean.cfc` — declares getFilename()
  - low `core/mura/content/file/fileBean.cfc` — declares getFilename()
- `categorybean → core/mura/category/categoryBean.cfc` — 1 finding(s), 6 candidate(s):
  - medium `core/mura/category/categoryBean.cfc` — declares getCategoryID(); named like the receiver 'categoryBean'
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getCategoryID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getCategoryID()
  - low `core/mura/extend/extendSet.cfc` — declares getCategoryID()
  - low `core/mura/user/userBean.cfc` — declares getCategoryID()
  - low `core/mura/user/userFeedBean.cfc` — declares getCategoryID()
- `categorynav[ct].nav → core/mura/bean/beanFeed.cfc` — 1 finding(s), 10 candidate(s):
  - low `core/mura/bean/beanFeed.cfc` — declares getIterator()
  - low `core/mura/bean/beanORM.cfc` — declares getIterator()
  - low `core/mura/category/categoryManager.cfc` — declares getIterator()
  - low `core/mura/content/contentManager.cfc` — declares getIterator()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getIterator()
  - low `core/mura/trash/trashManager.cfc` — declares getIterator()
  - low `core/mura/user/userFeedBean.cfc` — declares getIterator()
  - low `core/mura/user/userManager.cfc` — declares getIterator()
  - … 2 more
- `childcontentbean → core/mura/content/contentBean.cfc` — 1 finding(s), 4 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getContentID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getContentID()
  - low `core/mura/content/rater/rateBean.cfc` — declares getContentID()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getContentID()
- `commenter → core/mura/bean/beanORM.cfc` — 1 finding(s), 15 candidate(s):
  - low `core/mura/bean/beanORM.cfc` — declares loadBy()
  - low `core/mura/category/categoryBean.cfc` — declares loadBy()
  - low `core/mura/content/contentBean.cfc` — declares loadBy()
  - low `core/mura/content/contentCommentBean.cfc` — declares loadBy()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares loadBy()
  - low `core/mura/extend/extendObject.cfc` — declares loadBy()
  - low `core/mura/mailinglist/mailinglistBean.cfc` — declares loadBy()
  - low `core/mura/settings/settingsBean.cfc` — declares loadBy()
  - … 7 more
- `content → core/mura/bean/beanExtendable.cfc` — 1 finding(s), 14 candidate(s):
  - low `core/mura/bean/beanExtendable.cfc` — declares getType()
  - low `core/mura/content/contentBean.cfc` — declares getType()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares getType()
  - low `core/mura/extend/extendAttribute.cfc` — declares getType()
  - low `core/mura/extend/extendData.cfc` — declares getType()
  - low `core/mura/extend/extendObject.cfc` — declares getType()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getType()
  - low `core/mura/extend/extendSubType.cfc` — declares getType()
  - … 6 more
- `content → core/mura/bean/beanFeed.cfc` — 1 finding(s), 15 candidate(s):
  - low `core/mura/bean/beanFeed.cfc` — declares getQuery()
  - low `core/mura/content/contentCommentBean.cfc` — declares getQuery()
  - low `core/mura/content/contentStatsBean.cfc` — declares getQuery()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getQuery()
  - low `core/mura/iterator/queryIterator.cfc` — declares getQuery()
  - low `core/mura/plugin/pluginDisplayObjectBean.cfc` — declares getQuery()
  - low `core/mura/plugin/pluginScriptBean.cfc` — declares getQuery()
  - low `core/mura/settings/settingsImageSizeBean.cfc` — declares getQuery()
  - … 7 more
- `content → core/mura/configBean.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - … 17 more
- `content → core/mura/formBuilder/datarecordBean.cfc` — 1 finding(s), 13 candidate(s):
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue()
  - low `core/mura/formBuilder/fieldBean.cfc` — declares getValue()
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/event.cfc` — declares getValue()
  - low `core/mura/servletEvent.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - … 5 more
- `contentbean → core/mura/content/feed/feedBean.cfc` — 1 finding(s), 45 candidate(s):
  - low `core/mura/content/feed/feedBean.cfc` — declares save()
  - low `core/mura/content/feed/feedManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - low `core/mura/content/contentManager.cfc` — declares save()
  - low `core/mura/content/contentStatsBean.cfc` — declares save()
  - … 37 more
- `contentstats → core/mura/configBean.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - … 17 more
- `databean → core/mura/MasaScope.cfc` — 1 finding(s), 11 candidate(s):
  - low `core/mura/MasaScope.cfc` — declares getFeed()
  - low `core/mura/cfobject.cfc` — declares getFeed()
  - low `core/mura/bean/bean.cfc` — declares getFeed()
  - low `core/mura/bean/beanORM.cfc` — declares getFeed()
  - low `core/mura/iterator/queryIterator.cfc` — declares getFeed()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getFeed()
  - low `core/mura/content/changeset/changesetManager.cfc` — declares getFeed()
  - low `core/mura/content/feed/feedBean.cfc` — declares getFeed()
  - … 3 more
- `databean → core/mura/content/feed/feedBean.cfc` — 1 finding(s), 15 candidate(s):
  - medium `core/mura/content/feed/feedBean.cfc` — declares getQuery(); named like the receiver 'Feed'
  - low `core/mura/bean/beanFeed.cfc` — declares getQuery()
  - low `core/mura/content/contentCommentBean.cfc` — declares getQuery()
  - low `core/mura/content/contentStatsBean.cfc` — declares getQuery()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getQuery()
  - low `core/mura/iterator/queryIterator.cfc` — declares getQuery()
  - low `core/mura/plugin/pluginDisplayObjectBean.cfc` — declares getQuery()
  - low `core/mura/plugin/pluginScriptBean.cfc` — declares getQuery()
  - … 7 more
- `extendsets[] → core/mura/extend/extendAttribute.cfc` — 1 finding(s), 3 candidate(s):
  - low `core/mura/extend/extendAttribute.cfc` — declares getExtendSetID()
  - low `core/mura/extend/extendSet.cfc` — declares getExtendSetID()
  - low `core/mura/bean/beanExtendable.cfc` — declares getExtendSetID()
- `feed → core/mura/bean/bean.cfc` — 1 finding(s), 4 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares getTable()
  - low `core/mura/bean/beanFeed.cfc` — declares getTable()
  - low `core/mura/category/categoryFeedBean.cfc` — declares getTable()
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares getTable()
- `feed → core/mura/configBean.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - … 17 more
- `iterator → core/mura/utility.cfc` — 1 finding(s), 7 candidate(s):
  - low `core/mura/utility.cfc` — declares getNextN()
  - low `core/mura/bean/beanFeed.cfc` — declares getNextN()
  - low `core/mura/content/contentBean.cfc` — declares getNextN()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getNextN()
  - low `core/mura/iterator/queryIterator.cfc` — declares getNextN()
  - low `core/mura/settings/settingsBean.cfc` — declares getNextN()
  - low `core/mura/content/feed/feedBean.cfc` — declares getNextN()
- `kid → core/mura/category/categoryBean.cfc` — 1 finding(s), 45 candidate(s):
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - … 37 more
- `local.parentbean → core/mura/content/contentCommentFeedBean.cfc` — 1 finding(s), 26 candidate(s):
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares setSiteID()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares setSiteID()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares setSiteID()
  - low `core/mura/content/dataCollection/dataCollectionBean.cfc` — declares setSiteID()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares setSiteID()
  - low `core/mura/content/rater/rateBean.cfc` — declares setSiteID()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares setSiteID()
  - low `core/mura/bean/beanExtendable.cfc` — declares setSiteID()
  - … 18 more
- `local.parentbean → core/mura/content/feed/feedBean.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/content/feed/feedBean.cfc` — declares setParentID()
  - low `core/mura/formBuilder/datasetBean.cfc` — declares setParentID()
- `local.related → core/mura/bean/bean.cfc` — 1 finding(s), 11 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares getFeed()
  - low `core/mura/bean/beanORM.cfc` — declares getFeed()
  - low `core/mura/MasaScope.cfc` — declares getFeed()
  - low `core/mura/cfobject.cfc` — declares getFeed()
  - low `core/mura/iterator/queryIterator.cfc` — declares getFeed()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getFeed()
  - low `core/mura/content/changeset/changesetManager.cfc` — declares getFeed()
  - low `core/mura/content/feed/feedBean.cfc` — declares getFeed()
  - … 3 more
- `newcategorybean → core/mura/content/changeset/changesetBean.cfc` — 1 finding(s), 6 candidate(s):
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getCategoryID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getCategoryID()
  - low `core/mura/category/categoryBean.cfc` — declares getCategoryID()
  - low `core/mura/extend/extendSet.cfc` — declares getCategoryID()
  - low `core/mura/user/userBean.cfc` — declares getCategoryID()
  - low `core/mura/user/userFeedBean.cfc` — declares getCategoryID()
- `rc.content → core/mura/content/contentBean.cfc` — 1 finding(s), 2 candidate(s):
  - medium `core/mura/content/contentBean.cfc` — declares getCrumbArray(); named like the receiver 'content'
  - low `core/mura/content/contentNavBean.cfc` — declares getCrumbArray()
- `rc.extendsetbean → core/mura/extend/extendSet.cfc` — 1 finding(s), 6 candidate(s):
  - medium `core/mura/extend/extendSet.cfc` — declares getCategoryID(); named like the receiver 'extendSetBean'
  - low `core/mura/category/categoryBean.cfc` — declares getCategoryID()
  - low `core/mura/user/userBean.cfc` — declares getCategoryID()
  - low `core/mura/user/userFeedBean.cfc` — declares getCategoryID()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getCategoryID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getCategoryID()
- `rc.parentbean → core/mura/extend/extendRelatedContentSetBean.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/extend/extendRelatedContentSetBean.cfc` — declares getAvailableSubTypes()
  - low `core/mura/extend/extendSubType.cfc` — declares getAvailableSubTypes()
- `rc.subtypebean → core/mura/extend/extendRelatedContentSetBean.cfc` — 1 finding(s), 3 candidate(s):
  - low `core/mura/extend/extendRelatedContentSetBean.cfc` — declares getSubTypeID()
  - low `core/mura/extend/extendSet.cfc` — declares getSubTypeID()
  - low `core/mura/extend/extendSubType.cfc` — declares getSubTypeID()
- `rc.theimport.parentbean → core/mura/content/contentBean.cfc` — 1 finding(s), 4 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getcontentID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getcontentID()
  - low `core/mura/content/rater/rateBean.cfc` — declares getcontentID()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getcontentID()
- `redirectbean → core/mura/user/userBean.cfc` — 1 finding(s), 4 candidate(s):
  - low `core/mura/user/userBean.cfc` — declares getUserID()
  - low `core/mura/content/contentCommentBean.cfc` — declares getUserID()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getUserID()
  - low `core/mura/content/rater/rateBean.cfc` — declares getUserID()
- `relatedcontentsetarray[s] → core/mura/extend/extendAttribute.cfc` — 1 finding(s), 15 candidate(s):
  - low `core/mura/extend/extendAttribute.cfc` — declares setIsNew()
  - low `core/mura/extend/extendSet.cfc` — declares setIsNew()
  - low `core/mura/extend/extendSubType.cfc` — declares setIsNew()
  - low `core/mura/bean/bean.cfc` — declares setIsNew()
  - low `core/mura/category/categoryBean.cfc` — declares setIsNew()
  - low `core/mura/content/contentBean.cfc` — declares setIsNew()
  - low `core/mura/content/contentCommentBean.cfc` — declares setIsNew()
  - low `core/mura/mailinglist/mailinglistBean.cfc` — declares setIsNew()
  - … 7 more
- `relatedsetbean → core/mura/extend/extendAttribute.cfc` — 1 finding(s), 4 candidate(s):
  - low `core/mura/extend/extendAttribute.cfc` — declares getAsXML()
  - low `core/mura/extend/extendRelatedContentSetBean.cfc` — declares getAsXML()
  - low `core/mura/extend/extendSet.cfc` — declares getAsXML()
  - low `core/mura/extend/extendSubType.cfc` — declares getAsXML()
- `request.contentrenderer → core/mura/cfobject.cfc` — 1 finding(s), 13 candidate(s):
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/event.cfc` — declares getValue()
  - low `core/mura/servletEvent.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - low `core/mura/content/contentNavBean.cfc` — declares getValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue()
  - … 5 more
- `request.context.$ → core/mura/content/contentFileMetaDataBean.cfc` — 1 finding(s), 3 candidate(s):
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares getURLForImage()
  - low `core/mura/content/contentRenderer.cfc` — declares getURLForImage()
  - low `core/mura/client/api/json/v1/jsonApiUtility.cfc` — declares getURLForImage()
- `responseobject → core/mura/email/emailBean.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/email/emailBean.cfc` — declares setStatus()
  - low `core/mura/content/approval/approvalRequestBean.cfc` — declares setStatus()
- `retrieved → core/mura/trash/trashItemBean.cfc` — 1 finding(s), 10 candidate(s):
  - low `core/mura/trash/trashItemBean.cfc` — declares setAllValues()
  - low `core/mura/bean/bean.cfc` — declares setAllValues()
  - low `core/mura/content/contentBean.cfc` — declares setAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares setAllValues()
  - low `core/mura/extend/extendData.cfc` — declares setAllValues()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares setAllValues()
  - low `core/mura/formBuilder/datasetBean.cfc` — declares setAllValues()
  - low `core/mura/formBuilder/fieldBean.cfc` — declares setAllValues()
  - … 2 more
- `session.mura.editbean → core/mura/content/contentCommentBean.cfc` — 1 finding(s), 4 candidate(s):
  - low `core/mura/content/contentCommentBean.cfc` — declares getUserID()
  - low `core/mura/user/userBean.cfc` — declares getUserID()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getUserID()
  - low `core/mura/content/rater/rateBean.cfc` — declares getUserID()
- `sets[i] → core/mura/extend/extendAttribute.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/extend/extendAttribute.cfc` — declares getAllValues()
  - low `core/mura/extend/extendData.cfc` — declares getAllValues()
  - low `core/mura/extend/extendSet.cfc` — declares getAllValues()
  - low `core/mura/extend/extendSubType.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - … 17 more
- `st → core/mura/extend/extendManager.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/extend/extendManager.cfc` — declares getIconClass()
  - low `core/mura/extend/extendSubType.cfc` — declares getIconClass()
- `stats → admin/core/controllers/cchain.cfc` — 1 finding(s), 45 candidate(s):
  - low `admin/core/controllers/cchain.cfc` — declares save()
  - low `admin/core/controllers/cchangesets.cfc` — declares save()
  - low `admin/core/controllers/cwebservice.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - … 37 more
- `theimport.parentbean → core/mura/content/feed/feedBean.cfc` — 1 finding(s), 15 candidate(s):
  - low `core/mura/content/feed/feedBean.cfc` — declares getIsNew()
  - low `core/mura/content/contentBean.cfc` — declares getIsNew()
  - low `core/mura/content/contentCommentBean.cfc` — declares getIsNew()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getIsNew()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getIsNew()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getIsNew()
  - low `core/mura/bean/bean.cfc` — declares getIsNew()
  - low `core/mura/category/categoryBean.cfc` — declares getIsNew()
  - … 7 more
- `variables.bean → core/mura/bean/bean.cfc` — 1 finding(s), 25 candidate(s):
  - medium `core/mura/bean/bean.cfc` — declares getAllValues(); named like the receiver 'bean'
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - … 17 more
- `variables.collection → core/mura/cache/provider/cacheAdobe.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/cache/provider/cacheAdobe.cfc` — declares put()
  - low `core/mura/cache/provider/cacheLucee.cfc` — declares put()
- `variables.content → core/mura/content/contentBean.cfc` — 1 finding(s), 2 candidate(s):
  - medium `core/mura/content/contentBean.cfc` — declares setDisplayInterval(); named like the receiver 'content'
  - low `core/mura/content/contentNavBean.cfc` — declares setDisplayInterval()
- `variables.crumb → core/mura/bean/beanExtendable.cfc` — 1 finding(s), 14 candidate(s):
  - low `core/mura/bean/beanExtendable.cfc` — declares getType()
  - low `core/mura/content/contentBean.cfc` — declares getType()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares getType()
  - low `core/mura/extend/extendAttribute.cfc` — declares getType()
  - low `core/mura/extend/extendData.cfc` — declares getType()
  - low `core/mura/extend/extendObject.cfc` — declares getType()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getType()
  - low `core/mura/extend/extendSubType.cfc` — declares getType()
  - … 6 more
- `variables.map → core/mura/cache/cacheAbstract.cfc` — 1 finding(s), 3 candidate(s):
  - low `core/mura/cache/cacheAbstract.cfc` — declares size()
  - low `core/mura/cache/provider/cacheAdobe.cfc` — declares size()
  - low `core/mura/cache/provider/cacheLucee.cfc` — declares size()
- `variables.pluginconfigs[] → core/mura/plugin/pluginConfig.cfc` — 1 finding(s), 7 candidate(s):
  - low `core/mura/plugin/pluginConfig.cfc` — declares getModuleID()
  - low `core/mura/plugin/pluginDisplayObjectBean.cfc` — declares getModuleID()
  - low `core/mura/plugin/pluginScriptBean.cfc` — declares getModuleID()
  - low `core/mura/plugin/pluginSettingBean.cfc` — declares getModuleID()
  - low `core/mura/content/contentBean.cfc` — declares getModuleID()
  - low `core/mura/formBuilder/fieldtypeBean.cfc` — declares getModuleID()
  - low `core/mura/content/file/fileBean.cfc` — declares getModuleID()

</details>

## Return types — a call chained on a method that declares no component

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 55 | `method 'event' in mura.MuraScope has no component return type → core/mura/event.cfc` | 13 | medium | declares setValue(); named like the receiver 'event' (13 candidates) | `admin/core/controllers/carch.cfc:386` method 'event' in mura.MuraScope has no component return type (chain to 'setValue') |
| 35 | `method 'content' in mura.MuraScope has no component return type → core/mura/content/contentBean.cfc` | 1 | high | declares getStats(), requiresApproval(); named like the receiver 'content' | `admin/core/utilities/modal/toolbar.cfm:160` method 'content' in mura.MuraScope has no component return type (chain to 'getStats') |
| 33 | `method 'getClassExtensionManager' in configBean has no component return type → core/mura/extend/extendManager.cfc` | 1 | medium | declares loadConfigXML() | `core/appcfc/onApplicationStart_include.cfm:844` method 'getClassExtensionManager' in configBean has no component return type (chain to 'loadConfigXML') |
| 31 | `method 'getValue' in mura.servletEvent has no component return type → core/mura/content/contentBean.cfc` | 15 | low | declares getIsNew() (15 candidates) | `core/mura/content/contentRenderer.cfc:892` method 'getValue' in mura.servletEvent has no component return type (chain to 'getIsNew') |
| 24 | `method 'getBean' in MuraScope has no component return type → core/mura/bean/beanORM.cfc` | 15 | low | declares loadBy() (15 candidates) | `core/mura/client/api/json/v1/jsonApiUtility.cfc:2048` method 'getBean' in MuraScope has no component return type (chain to 'loadBy') |
| 24 | `method 'getContentRenderer' in settingsBean has no component return type → core/mura/content/contentRenderer.cfc` | 1 | high | declares useLayoutManager(); named like the receiver 'ContentRenderer' | `admin/core/views/carch/form/dsp_panel_publishing.cfm:313` method 'getContentRenderer' in settingsBean has no component return type (chain to 'useLayoutManager') |
| 23 | `method 'getFeed' in mura.MuraScope has no component return type → core/mura/bean/beanFeed.cfc` | 10 | low | declares getIterator() (10 candidates) | `admin/core/views/cextend/editrelatedcontentset.cfm:103` method 'getFeed' in mura.MuraScope has no component return type (chain to 'getIterator') |
| 17 | `method 'getContentRenderer' in mura.MuraScope has no component return type → core/mura/content/contentRenderer.cfc` | 1 | high | declares useLayoutmanager(); named like the receiver 'ContentRenderer' | `admin/core/views/carch/dsp_close_compact_display.cfm:141` method 'getContentRenderer' in mura.MuraScope has no component return type (chain to 'useLayoutmanager') |
| 16 | `method 'content' in mura.MuraScope has no component return type → core/mura/bean/beanExtendable.cfc` | 6 | low | declares getSubType(), getType() (6 candidates) | `core/mura/content/contentRendererUtility.cfc:1986` method 'content' in mura.MuraScope has no component return type (chain to 'getSubType') |
| 16 | `method 'getFormBean' has no component return type → core/mura/content/contentBean.cfc` | 1 | medium | declares getBody() | `core/mura/content/dataCollection/dataCollectionBean.cfc:223` method 'getFormBean' has no component return type (chain to 'getBody') |
| 15 | `method 'getRBFactory' in settingsBean has no component return type → core/mura/googleAuth.cfc` | 2 | low | declares getKey() (2 candidates) | `core/mura/bean/beanValidator.cfc:245` method 'getRBFactory' in settingsBean has no component return type (chain to 'getKey') |
| 11 | `method 'getFeed' has no component return type → core/mura/bean/beanFeed.cfc` | 1 | medium | declares isEQ() | `core/mura/user/userDAO.cfc:365` method 'getFeed' has no component return type (chain to 'isEQ') |
| 10 | `method 'event' in MuraScope has no component return type → core/mura/event.cfc` | 13 | medium | declares setValue(); named like the receiver 'event' (13 candidates) | `core/appcfc/onRequestEnd_include.cfm:90` method 'event' in MuraScope has no component return type (chain to 'setValue') |
| 10 | `method 'getApi' in settingsBean has no component return type → core/mura/settings/settingsBean.cfc` | 3 | low | declares getEndpoint() (3 candidates) | `admin/core/views/cfeed/edit.cfm:215` method 'getApi' in settingsBean has no component return type (chain to 'getEndpoint') |
| 10 | `method 'getBean' has no component return type → core/mura/bean/bean.cfc` | 11 | medium | declares getFeed(); named like the receiver 'Bean' (11 candidates) | `core/mura/MasaScope.cfc:583` method 'getBean' has no component return type (chain to 'getFeed') |
| 9 | `method 'getClassExtensionManager' in mura.configBean has no component return type → core/mura/extend/extendManager.cfc` | 1 | medium | declares getSubTypes() | `admin/core/views/carch/loadsiteflat.cfm:90` method 'getClassExtensionManager' in mura.configBean has no component return type (chain to 'getSubTypes') |
| 8 | `method 'getFeed' in MuraScope has no component return type → core/mura/bean/beanFeed.cfc` | 1 | high | declares aggregate(), addParam(), getIterator() | `core/tests/specs/mura/core/feedDenylistGenericSecurity.cfc:26` method 'getFeed' in MuraScope has no component return type (chain to 'aggregate') |
| 8 | `method 'getFeed' in categoryBean has no component return type → core/mura/bean/beanFeed.cfc` | 1 | medium | declares isEQ() | `core/mura/content/contentCategoryAssignBean.cfc:102` method 'getFeed' in categoryBean has no component return type (chain to 'isEQ') |
| 8 | `method 'loadBy' in fileBean has no component return type → core/mura/content/contentBean.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/content/contentFileMetaDataBean.cfc:210` method 'loadBy' in fileBean has no component return type (chain to 'save') |
| 7 | `method 'event' in mura.MuraScope has no component return type → core/mura/user/userBean.cfc` | 4 | high | declares getErrors(), getInactive(), getUserID() | `core/mura/client/api/json/v1/jsonApiUtility.cfc:3973` method 'event' in mura.MuraScope has no component return type (chain to 'getErrors') |
| 7 | `method 'getContentRenderer' has no component return type → core/mura/content/contentRenderer.cfc` | 2 | medium | declares getCurrentURL(); named like the receiver 'ContentRenderer' (2 candidates) | `core/mura/MasaScope.cfc:371` method 'getContentRenderer' has no component return type (chain to 'getCurrentURL') |
| 7 | `method 'getRBFactory' in settingsBean has no component return type → core/mura/resourceBundle/resourceBundleFactory.cfc` | 1 | medium | declares isSupportedLocale() | `admin/core/controllers/csettings.cfc:214` method 'getRBFactory' in settingsBean has no component return type (chain to 'isSupportedLocale') |
| 7 | `method 'getRazunaSettings' in settingsBean has no component return type → core/mura/content/file/razuna/razunaSettingsBean.cfc` | 1 | high | declares getApiKey(), getServerType(); named like the receiver 'RazunaSettings' | `admin/common/layouts/includes/dialog.cfm:80` method 'getRazunaSettings' in settingsBean has no component return type (chain to 'getApiKey') |
| 7 | `method 'getSite' in mura.servletEvent has no component return type → core/mura/settings/settingsBean.cfc` | 1 | medium | declares getIncludePath() | `core/mura/content/contentRenderer.cfc:1480` method 'getSite' in mura.servletEvent has no component return type (chain to 'getIncludePath') |
| 6 | `method 'event' in mura.MuraScope has no component return type → core/mura/plugin/pluginStandardEventWrapper.cfc` | 1 | medium | declares handle() | `core/mura/Handler/standardEventsHandler.cfc:724` method 'event' in mura.MuraScope has no component return type (chain to 'handle') |
| 6 | `method 'getContentBean' has no component return type → core/mura/content/contentNavBean.cfc` | 13 | low | declares getValue() (13 candidates) | `core/mura/content/contentNavBean.cfc:150` method 'getContentBean' has no component return type (chain to 'getValue') |
| 6 | `method 'getSite' has no component return type → core/mura/settings/settingsBean.cfc` | 1 | medium | declares getRbFactory() | `core/modules/v1/nav/dsp_tag_cloud.cfm:103` method 'getSite' has no component return type (chain to 'getRbFactory') |
| 6 | `method 'loadBy' in approvalActionBean has no component return type → core/mura/content/approval/approvalActionBean.cfc` | 1 | medium | declares setActionType() | `core/mura/content/approval/approvalRequestBean.cfc:21` method 'loadBy' in approvalActionBean has no component return type (chain to 'setActionType') |
| 6 | `method 'loadBy' in approvalActionBean has no component return type → core/mura/content/approval/approvalChainBean.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/content/approval/approvalRequestBean.cfc:21` method 'loadBy' in approvalActionBean has no component return type (chain to 'save') |
| 6 | `method 'loadBy' in contentBean has no component return type → core/mura/content/contentBean.cfc` | 2 | low | declares getCrumbArray() (2 candidates) | `admin/core/views/carch/import.cfm:131` method 'loadBy' in contentBean has no component return type (chain to 'getCrumbArray') |
| 5 | `method 'getApi' in settingsBean has no component return type → core/mura/client/api/feed/v1/feedApiUtility.cfc` | 3 | low | declares getEndpoint() (3 candidates) | `core/mura/client/api/resource/variation.js.cfm:25` method 'getApi' in settingsBean has no component return type (chain to 'getEndpoint') |
| 5 | `method 'getBean' has no component return type → core/mura/bean/beanORM.cfc` | 15 | low | declares loadBy() (15 candidates) | `core/mura/bean/beanORM.cfc:625` method 'getBean' has no component return type (chain to 'loadBy') |
| 5 | `method 'getBean' in mura.MuraScope has no component return type → core/mura/bean/beanORM.cfc` | 2 | low | declares checkSchema() (2 candidates) | `core/mura/formBuilder/formBuilderManager.cfc:522` method 'getBean' in mura.MuraScope has no component return type (chain to 'checkSchema') |
| 5 | `method 'getContentRenderer' in MuraScope has no component return type → core/mura/content/contentRenderer.cfc` | 1 | high | declares useLayoutManager(); named like the receiver 'ContentRenderer' | `admin/assets/js/frontendtools.js.cfm:22` method 'getContentRenderer' in MuraScope has no component return type (chain to 'useLayoutManager') |
| 5 | `method 'getValue' in mura.servletEvent has no component return type` | 3 | none |  | `core/mura/content/contentRenderer.cfc:1366` method 'getValue' in mura.servletEvent has no component return type (chain to 'getparentid') |
| 5 | `method 'siteConfig' in mura.MuraScope has no component return type → core/modules/v1/filebrowser/model/beans/filebrowser.cfc` | 3 | low | declares getResourceBundle() (3 candidates) | `core/modules/v1/comments/index.cfm:404` method 'siteConfig' in mura.MuraScope has no component return type (chain to 'getResourceBundle') |
| 5 | `method 'siteConfig' in mura.MuraScope has no component return type → core/mura/resourceBundle/resourceBundle.cfc` | 1 | high | declares messageFormat(); named like the receiver 'ResourceBundle' | `core/modules/v1/comments/index.cfm:404` method 'siteConfig' in mura.MuraScope has no component return type (chain to 'messageFormat') |
| 4 | `method 'event' in MuraScope has no component return type → core/mura/content/contentCommentBean.cfc` | 6 | low | declares getEmail() (6 candidates) | `core/mura/content/approval/approvalRequestBean.cfc:201` method 'event' in MuraScope has no component return type (chain to 'getEmail') |
| 4 | `method 'getAPI' in settingsBean has no component return type → core/mura/client/api/json/v1/jsonApiUtility.cfc` | 1 | medium | declares getSerializer() | `core/modules/v1/htmlhead/global.cfm:103` method 'getAPI' in settingsBean has no component return type (chain to 'getSerializer') |
| 4 | `method 'getBean' has no component return type → core/mura/content/contentBean.cfc` | 9 | low | declares setOrderNo() (9 candidates) | `core/mura/bean/beanORM.cfc:644` method 'getBean' has no component return type (chain to 'setOrderNo') |
| 4 | `method 'getCacheFactory' in settingsBean has no component return type → core/mura/cache/cacheAbstract.cfc` | 4 | low | declares purgeAll() (4 candidates) | `core/mura/settings/settingsManager.cfc:584` method 'getCacheFactory' in settingsBean has no component return type (chain to 'purgeAll') |
| 4 | `method 'getClassExtensionManager' in configBean has no component return type → core/mura/extend/extendSubType.cfc` | 1 | medium | declares getHasConfigurator() | `core/modules/v1/calendar/configurator.cfm:110` method 'getClassExtensionManager' in configBean has no component return type (chain to 'getHasConfigurator') |
| 4 | `method 'getExtendedData' has no component return type → core/mura/extend/extendData.cfc` | 1 | medium | declares getAllExtendSetData() | `core/mura/bean/beanExtendable.cfc:174` method 'getExtendedData' has no component return type (chain to 'getAllExtendSetData') |
| 4 | `method 'loadBy' in extendRelatedContentSetBean has no component return type → core/mura/bean/beanRemotePointer.cfc` | 2 | low | declares getEntityType() (2 candidates) | `core/mura/content/contentManager.cfc:2233` method 'loadBy' in extendRelatedContentSetBean has no component return type (chain to 'getEntityType') |
| 3 | `method 'event' in MuraScope has no component return type → core/mura/user/userBean.cfc` | 6 | high | declares getEmail(), getMembersIterator() | `core/mura/content/approval/approvalRequestBean.cfc:210` method 'event' in MuraScope has no component return type (chain to 'getEmail') |
| 3 | `method 'getBean' in MuraScope has no component return type → core/mura/bean/beanEntity.cfc` | 45 | low | declares save() (45 candidates) | `core/tests/specs/mura/core/contentOrdering.cfc:74` method 'getBean' in MuraScope has no component return type (chain to 'save') |
| 3 | `method 'getContentBean' has no component return type → core/mura/cfobject.cfc` | 7 | low | declares valueExists() (7 candidates) | `core/mura/MasaScope.cfc:121` method 'getContentBean' has no component return type (chain to 'valueExists') |
| 3 | `method 'getContentBean' has no component return type → core/mura/content/contentBean.cfc` | 9 | medium | declares getParent(); named like the receiver 'ContentBean' (9 candidates) | `core/mura/MasaScope.cfc:379` method 'getContentBean' has no component return type (chain to 'getParent') |
| 3 | `method 'getEntity' has no component return type → core/mura/bean/bean.cfc` | 2 | low | declares getDbUtility() (2 candidates) | `core/mura/bean/beanFeed.cfc:236` method 'getEntity' has no component return type (chain to 'getDbUtility') |
| 3 | `method 'getEntity' has no component return type → core/mura/bean/beanORM.cfc` | 2 | low | declares getLoadSQLColumnsAndTables(), getPrimaryKey() (2 candidates) | `core/mura/bean/beanFeed.cfc:927` method 'getEntity' has no component return type (chain to 'getLoadSQLColumnsAndTables') |
| 3 | `method 'getFeed' has no component return type → core/mura/user/userFeedBean.cfc` | 10 | low | declares getIterator() (10 candidates) | `core/mura/user/userDAO.cfc:365` method 'getFeed' has no component return type (chain to 'getIterator') |
| 3 | `method 'loadBy' in approvalActionBean has no component return type → core/mura/content/contentCommentBean.cfc` | 4 | low | declares setUserID() (4 candidates) | `core/mura/content/approval/approvalRequestBean.cfc:21` method 'loadBy' in approvalActionBean has no component return type (chain to 'setUserID') |
| 3 | `method 'loadBy' in approvalChainBean has no component return type → core/mura/user/userBean.cfc` | 1 | medium | declares getMembershipsIterator() | `core/mura/content/approval/approvalChainBean.cfc:59` method 'loadBy' in approvalChainBean has no component return type (chain to 'getMembershipsIterator') |
| 3 | `method 'loadBy' in fileBean has no component return type → core/mura/content/contentFileMetaDataBean.cfc` | 2 | low | declares setAltText() (2 candidates) | `core/mura/content/contentFileMetaDataBean.cfc:210` method 'loadBy' in fileBean has no component return type (chain to 'setAltText') |
| 2 | `method 'content' in MuraScope has no component return type → core/mura/content/contentBean.cfc` | 15 | medium | declares getIsNew(); named like the receiver 'content' (15 candidates) | `admin/core/views/carch/edit_panels.cfm:334` method 'content' in MuraScope has no component return type (chain to 'getIsNew') |
| 2 | `method 'content' in mura.MuraScope has no component return type` | 25 | none |  | `core/mura/Handler/standardEventsHandler.cfc:847` method 'content' in mura.MuraScope has no component return type (chain to 'getAllValues') |
| 2 | `method 'event' has no component return type → core/mura/content/contentBean.cfc` | 4 | low | declares getContentID() (4 candidates) | `core/mura/MasaScope.cfc:399` method 'event' has no component return type (chain to 'getContentID') |
| 2 | `method 'event' has no component return type → core/mura/content/contentRenderer.cfc` | 13 | high | declares init(), setValue(), postMergeInit() | `core/mura/MasaScope.cfc:185` method 'event' has no component return type (chain to 'setValue') |
| 2 | `method 'event' has no component return type → core/mura/event.cfc` | 13 | medium | declares getValue(); named like the receiver 'event' (13 candidates) | `core/mura/MasaScope.cfc:399` method 'event' has no component return type (chain to 'getValue') |
| 2 | `method 'getActiveContent' in contentManager has no component return type → core/mura/content/contentBean.cfc` | 6 | low | declares getURL() (6 candidates) | `admin/core/views/carch/dsp_secondary_menu.cfm:379` method 'getActiveContent' in contentManager has no component return type (chain to 'getURL') |
| 2 | `method 'getApi' in settingsBean has no component return type → core/mura/client/api/json/v1/jsonApiUtility.cfc` | 1 | medium | declares registerEntity() | `core/mura/configBean.cfc:2045` method 'getApi' in settingsBean has no component return type (chain to 'registerEntity') |
| 2 | `method 'getClassExtensionManager' has no component return type → core/mura/extend/extendManager.cfc` | 1 | medium | declares resetTypedData() | `core/mura/dbUpdates/5.2.2655.cfm:184` method 'getClassExtensionManager' has no component return type (chain to 'resetTypedData') |
| 2 | `method 'getClassExtensionManager' in configBean has no component return type → core/mura/bean/beanExtendable.cfc` | 2 | low | declares getExtendedData() (2 candidates) | `core/mura/bean/beanExtendable.cfc:141` method 'getClassExtensionManager' in configBean has no component return type (chain to 'getExtendedData') |
| 2 | `method 'getContentRenderer' in muraScope has no component return type → core/mura/content/contentRenderer.cfc` | 1 | high | declares useLayoutManager(); named like the receiver 'ContentRenderer' | `core/mura/customtags/objectconfigurator.cfm:114` method 'getContentRenderer' in muraScope has no component return type (chain to 'useLayoutManager') |
| 2 | `method 'getDefaultBeanFactory' has no component return type → core/mura/bean/beanFactory.cfc` | 4 | low | declares containsBean(), getBean() (3 candidates) | `admin/framework.cfc:1380` method 'getDefaultBeanFactory' has no component return type (chain to 'containsBean') |
| 2 | `method 'getDisplayInterval' in contentBean has no component return type → core/mura/content/contentBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/content/contentIntervalManager.cfc:33` method 'getDisplayInterval' in contentBean has no component return type (chain to 'getAllValues') |
| 2 | `method 'getEvent' has no component return type → core/mura/event.cfc` | 13 | medium | declares setValue(), getValue(); named like the receiver 'Event' (13 candidates) | `core/mura/MasaScope.cfc:261` method 'getEvent' has no component return type (chain to 'setValue') |
| 2 | `method 'getFeed' in mura.MuraScope has no component return type → core/mura/content/feed/feedBean.cfc` | 4 | medium | declares setIsPublic(); named like the receiver 'Feed' (4 candidates) | `core/modules/v1/login/model/oauthLoginUtility.cfc:14` method 'getFeed' in mura.MuraScope has no component return type (chain to 'setIsPublic') |
| 2 | `method 'getHandler' in mura.event has no component return type → core/mura/plugin/pluginStandardEventWrapper.cfc` | 1 | medium | declares handle() | `core/mura/content/contentCommentBean.cfc:273` method 'getHandler' in mura.event has no component return type (chain to 'handle') |
| 2 | `method 'getLocaleUtils' has no component return type → core/mura/settings/settingsBean.cfc` | 2 | low | declares getJSDateKey() (2 candidates) | `core/mura/settings/settingsBean.cfc:727` method 'getLocaleUtils' has no component return type (chain to 'getJSDateKey') |
| 2 | `method 'getRazunaSettings' in settingsBean has no component return type → core/mura/settings/settingsBean.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/settings/settingsManager.cfc:271` method 'getRazunaSettings' in settingsBean has no component return type (chain to 'save') |
| 2 | `method 'getRelatedFeed' has no component return type → core/mura/bean/beanFeed.cfc` | 15 | low | declares getQuery() (15 candidates) | `admin/core/views/carch/loadrelatedcontent.cfm:407` method 'getRelatedFeed' has no component return type (chain to 'getQuery') |
| 2 | `method 'getSite' has no component return type → core/mura/googleAuth.cfc` | 2 | low | declares getKey() (2 candidates) | `core/mura/content/contentRenderer.cfc:1389` method 'getSite' has no component return type (chain to 'getKey') |
| 2 | `method 'getSite' in mura.servletEvent has no component return type → core/mura/configBean.cfc` | 2 | low | declares getAssetPath() (2 candidates) | `core/mura/content/contentRenderer.cfc:2831` method 'getSite' in mura.servletEvent has no component return type (chain to 'getAssetPath') |
| 2 | `method 'getSubsystemBeanFactory' has no component return type → core/mura/bean/beanFactory.cfc` | 4 | low | declares containsBean(), getBean() (3 candidates) | `admin/framework.cfc:1375` method 'getSubsystemBeanFactory' has no component return type (chain to 'containsBean') |
| 2 | `method 'getUserBean' has no component return type → core/mura/user/sessionUserFacade.cfc` | 13 | low | declares getValue() (13 candidates) | `core/mura/user/sessionUserFacade.cfc:54` method 'getUserBean' has no component return type (chain to 'getValue') |
| 2 | `method 'loadBy' in categoryBean has no component return type → core/mura/content/changeset/changesetBean.cfc` | 6 | low | declares getCategoryID() (6 candidates) | `core/mura/content/contentBean.cfc:912` method 'loadBy' in categoryBean has no component return type (chain to 'getCategoryID') |
| 2 | `method 'loadBy' in contentBean has no component return type → core/mura/bean/bean.cfc` | 4 | low | declares exists() (4 candidates) | `admin/core/views/carch/dsp_close_compact_display.cfm:109` method 'loadBy' in contentBean has no component return type (chain to 'exists') |
| 2 | `method 'loadBy' in contentBean has no component return type → core/mura/configBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `admin/core/controllers/carch.cfc:158` method 'loadBy' in contentBean has no component return type (chain to 'getAllValues') |
| 2 | `method 'loadBy' in contentFilenameArchiveBean has no component return type → core/mura/content/contentBean.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/content/contentManager.cfc:1695` method 'loadBy' in contentFilenameArchiveBean has no component return type (chain to 'save') |
| 2 | `method 'siteConfig' has no component return type → core/mura/resourceBundle/resourceBundleFactory.cfc` | 2 | high | declares getKeyValue(), getKey() | `core/mura/MasaScope.cfc:429` method 'siteConfig' has no component return type (chain to 'getKeyValue') |
| 1 | `method 'content' has no component return type → core/mura/content/contentBean.cfc` | 4 | medium | declares getCrumbIterator(); named like the receiver 'content' (4 candidates) | `core/mura/MasaScope.cfc:526` method 'content' has no component return type (chain to 'getCrumbIterator') |
| 1 | `method 'getApprovalRequest' in contentBean has no component return type → core/mura/content/approval/approvalRequestBean.cfc` | 2 | medium | declares setStatus(); named like the receiver 'ApprovalRequest' (2 candidates) | `core/mura/content/contentManager.cfc:1104` method 'getApprovalRequest' in contentBean has no component return type (chain to 'setStatus') |
| 1 | `method 'getBean' has no component return type → core/mura/bean/beanEntity.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/bean/beanORM.cfc:644` method 'getBean' has no component return type (chain to 'save') |
| 1 | `method 'getBean' has no component return type → core/mura/bean/ioc.cfc` | 2 | medium | declares onLoad() | `core/mura/bean/ioc.cfc:687` method 'getBean' has no component return type (chain to 'onLoad') |
| 1 | `method 'getBean' has no component return type → core/mura/client/api/json/v1/jsonApiUtility.cfc` | 2 | low | declares checkSchema() (2 candidates) | `core/mura/client/api/json/v1/jsonApiUtility.cfc:215` method 'getBean' has no component return type (chain to 'checkSchema') |
| 1 | `method 'getCache' has no component return type → core/mura/cache/cacheAbstract.cfc` | 4 | low | declares purge() (4 candidates) | `core/mura/bean/beanORM.cfc:1018` method 'getCache' has no component return type (chain to 'purge') |
| 1 | `method 'getCacheFactory' has no component return type → core/mura/cache/cacheAbstract.cfc` | 4 | low | declares purgeAll() (4 candidates) | `core/mura/settings/settingsBean.cfc:640` method 'getCacheFactory' has no component return type (chain to 'purgeAll') |
| 1 | `method 'getClassExtensionManager' in configBean has no component return type → core/mura/extend/extendData.cfc` | 2 | low | declares getDefinitionsQuery() (2 candidates) | `core/mura/extend/extendData.cfc:102` method 'getClassExtensionManager' in configBean has no component return type (chain to 'getDefinitionsQuery') |
| 1 | `method 'getConfig' has no component return type → core/mura/plugin/pluginConfig.cfc` | 2 | low | declares getAssignedSites() (2 candidates) | `core/mura/plugin/pluginManager.cfc:3285` method 'getConfig' has no component return type (chain to 'getAssignedSites') |
| 1 | `method 'getConfig' has no component return type → core/mura/plugin/pluginManager.cfc` | 4 | low | declares discoverBeans() (3 candidates) | `core/mura/plugin/pluginManager.cfc:3385` method 'getConfig' has no component return type (chain to 'discoverBeans') |
| 1 | `method 'getContentBean' in mura.servletEvent has no component return type → core/mura/content/contentBean.cfc` | 4 | medium | declares getContentID(); named like the receiver 'ContentBean' (4 candidates) | `core/mura/content/contentRenderer.cfc:1344` method 'getContentBean' in mura.servletEvent has no component return type (chain to 'getContentID') |
| 1 | `method 'getContentRenderer' in MuraScope has no component return type → core/mura/cfobject.cfc` | 1 | medium | declares inject() | `core/tests/specs/mura/core/contentTypes.cfc:27` method 'getContentRenderer' in MuraScope has no component return type (chain to 'inject') |
| 1 | `method 'getContentRenderer' in mura.MuraScope has no component return type → core/mura/cfobject.cfc` | 1 | medium | declares injectMethod() | `core/mura/client/api/json/v1/jsonApiUtility.cfc:3874` method 'getContentRenderer' in mura.MuraScope has no component return type (chain to 'injectMethod') |
| 1 | `method 'getContentRenderer' in settingsBean has no component return type → core/mura/content/contentFileMetaDataBean.cfc` | 1 | medium | declares getDirectImages() | `core/mura/content/contentFileMetaDataBean.cfc:159` method 'getContentRenderer' in settingsBean has no component return type (chain to 'getDirectImages') |
| 1 | `method 'getDisplayInterval' in contentBean has no component return type → core/mura/configBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `admin/core/views/carch/form/dsp_displaycontent.cfm:107` method 'getDisplayInterval' in contentBean has no component return type (chain to 'getAllValues') |
| 1 | `method 'getEntity' has no component return type → core/mura/dbUtility.cfc` | 1 | high | declares columns(); named like the receiver 'DbUtility' | `core/mura/bean/beanFeed.cfc:236` method 'getEntity' has no component return type (chain to 'columns') |
| 1 | `method 'getExtendedData' has no component return type → core/mura/bean/bean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/bean/beanExtendable.cfc:204` method 'getExtendedData' has no component return type (chain to 'getAllValues') |
| 1 | `method 'getExtendedData' in contentBean has no component return type → core/mura/content/contentBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/content/contentUtility.cfc:1714` method 'getExtendedData' in contentBean has no component return type (chain to 'getAllValues') |
| 1 | `method 'getFeed' in categoryBean has no component return type → core/mura/content/contentCommentFeedBean.cfc` | 26 | low | declares setSiteID() (26 candidates) | `core/mura/content/feed/feedBean.cfc:558` method 'getFeed' in categoryBean has no component return type (chain to 'setSiteID') |
| 1 | `method 'getFeed' in categoryBean has no component return type → core/mura/content/contentManager.cfc` | 10 | low | declares getIterator() (10 candidates) | `core/mura/content/contentCategoryAssignBean.cfc:102` method 'getFeed' in categoryBean has no component return type (chain to 'getIterator') |
| 1 | `method 'getFeed' in categoryBean has no component return type → core/mura/content/feed/feedBean.cfc` | 15 | low | declares getQuery() (15 candidates) | `core/mura/content/feed/feedBean.cfc:558` method 'getFeed' in categoryBean has no component return type (chain to 'getQuery') |
| 1 | `method 'getFeed' in mura.MuraScope has no component return type → core/mura/bean/beanExtendable.cfc` | 26 | low | declares setSiteID() (26 candidates) | `admin/core/views/cwebservice/list.cfm:78` method 'getFeed' in mura.MuraScope has no component return type (chain to 'setSiteID') |
| 1 | `method 'getKidsIterator' has no component return type → core/mura/content/contentCommentBean.cfc` | 15 | low | declares getQuery() (15 candidates) | `core/mura/content/contentCategoryAssignBean.cfc:110` method 'getKidsIterator' has no component return type (chain to 'getQuery') |
| 1 | `method 'getManager' has no component return type → core/mura/extend/extendAttribute.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/extend/extendObject.cfc:130` method 'getManager' has no component return type (chain to 'save') |
| 1 | `method 'getObject' in trashItemBean has no component return type → core/mura/bean/beanEntity.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/trash/trashManager.cfc:439` method 'getObject' in trashItemBean has no component return type (chain to 'save') |
| 1 | `method 'getParent' in contentBean has no component return type → core/mura/bean/beanExtendable.cfc` | 14 | low | declares getType() (14 candidates) | `admin/core/views/carch/form/dsp_displaycontent.cfm:77` method 'getParent' in contentBean has no component return type (chain to 'getType') |
| 1 | `method 'getParent' in mura.MuraScope has no component return type → core/mura/content/contentBean.cfc` | 14 | low | declares getType() (14 candidates) | `core/mura/content/contentRenderer.cfc:1841` method 'getParent' in mura.MuraScope has no component return type (chain to 'getType') |
| 1 | `method 'getRBFactory' has no component return type → core/mura/resourceBundle/resourceBundle.cfc` | 2 | low | declares getUtils() (2 candidates) | `core/mura/settings/settingsBean.cfc:740` method 'getRBFactory' has no component return type (chain to 'getUtils') |
| 1 | `method 'getRBFactory' in settingsBean has no component return type → core/mura/resourceBundle/resourceBundle.cfc` | 1 | high | declares messageFormat(); named like the receiver 'ResourceBundle' | `core/mura/user/userBean.cfc:478` method 'getRBFactory' in settingsBean has no component return type (chain to 'messageFormat') |
| 1 | `method 'getSiteRenderer' in mura.MuraScope has no component return type` | 0 | none |  | `core/modules/v1/nav/calendarNav/navTools.cfc:206` method 'getSiteRenderer' in mura.MuraScope has no component return type (chain to 'getNavCalendarTableClass') |
| 1 | `method 'getUserBean' has no component return type → core/mura/user/userBean.cfc` | 25 | medium | declares getAllValues(); named like the receiver 'UserBean' (25 candidates) | `core/mura/user/sessionUserFacade.cfc:203` method 'getUserBean' has no component return type (chain to 'getAllValues') |
| 1 | `method 'getcontentRenderer' in settingsBean has no component return type → core/mura/content/contentRenderer.cfc` | 2 | medium | declares getURLStem(); named like the receiver 'contentRenderer' (2 candidates) | `core/mura/email/emailUtility.cfc:205` method 'getcontentRenderer' in settingsBean has no component return type (chain to 'getURLStem') |
| 1 | `method 'loadBy' in approvalChainAssignmentBean has no component return type → admin/core/controllers/cchain.cfc` | 45 | low | declares save() (45 candidates) | `admin/core/controllers/cperm.cfc:101` method 'loadBy' in approvalChainAssignmentBean has no component return type (chain to 'save') |
| 1 | `method 'loadBy' in approvalChainAssignmentBean has no component return type → core/mura/content/approval/approvalChainAssignmentBean.cfc` | 1 | medium | declares setExemptID() | `admin/core/controllers/cperm.cfc:101` method 'loadBy' in approvalChainAssignmentBean has no component return type (chain to 'setExemptID') |
| 1 | `method 'loadBy' in approvalChainAssignmentBean has no component return type → core/mura/content/approval/approvalChainBean.cfc` | 1 | medium | declares setChainID() | `admin/core/controllers/cperm.cfc:101` method 'loadBy' in approvalChainAssignmentBean has no component return type (chain to 'setChainID') |
| 1 | `method 'loadBy' in approvalChainBean has no component return type → admin/core/controllers/cchain.cfc` | 45 | low | declares save() (45 candidates) | `admin/core/controllers/cchain.cfc:108` method 'loadBy' in approvalChainBean has no component return type (chain to 'save') |
| 1 | `method 'loadBy' in approvalChainMembershipBean has no component return type → core/mura/content/approval/approvalChainBean.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/content/approval/approvalChainBean.cfc:75` method 'loadBy' in approvalChainMembershipBean has no component return type (chain to 'save') |
| 1 | `method 'loadBy' in approvalChainMembershipBean has no component return type → core/mura/content/approval/approvalChainMembershipBean.cfc` | 9 | low | declares setOrderNo() (9 candidates) | `core/mura/content/approval/approvalChainBean.cfc:75` method 'loadBy' in approvalChainMembershipBean has no component return type (chain to 'setOrderNo') |
| 1 | `method 'loadBy' in approvalRequestBean has no component return type → core/mura/content/approval/approvalRequestBean.cfc` | 1 | medium | declares cancel() | `core/mura/content/contentManager.cfc:1689` method 'loadBy' in approvalRequestBean has no component return type (chain to 'cancel') |
| 1 | `method 'loadBy' in approvalRequestBean has no component return type → core/mura/content/changeset/changesetBean.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/content/changeset/changesetManager.cfc:536` method 'loadBy' in approvalRequestBean has no component return type (chain to 'save') |
| 1 | `method 'loadBy' in approvalRequestBean has no component return type → core/mura/content/contentBean.cfc` | 1 | medium | declares setContentHistID() | `core/mura/content/changeset/changesetManager.cfc:536` method 'loadBy' in approvalRequestBean has no component return type (chain to 'setContentHistID') |
| 1 | `method 'loadBy' in beanEntity has no component return type → core/mura/extend/extendRelatedContentSetBean.cfc` | 3 | low | declares getDisplayName() (3 candidates) | `core/mura/extend/extendRelatedContentSetBean.cfc:26` method 'loadBy' in beanEntity has no component return type (chain to 'getDisplayName') |
| 1 | `method 'loadBy' in categoryBean has no component return type → core/mura/content/feed/feedBean.cfc` | 6 | low | declares getCategoryID() (6 candidates) | `core/mura/content/feed/feedBean.cfc:568` method 'loadBy' in categoryBean has no component return type (chain to 'getCategoryID') |
| 1 | `method 'loadBy' in changesetCategoryAssignmentBean has no component return type → core/mura/content/changeset/changesetBean.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/content/changeset/changesetManager.cfc:203` method 'loadBy' in changesetCategoryAssignmentBean has no component return type (chain to 'save') |
| 1 | `method 'loadBy' in changesetRollBackBean has no component return type → core/mura/content/changeset/changesetBean.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/content/changeset/changesetManager.cfc:544` method 'loadBy' in changesetRollBackBean has no component return type (chain to 'save') |
| 1 | `method 'loadBy' in changesetTagAssignmentBean has no component return type → core/mura/content/changeset/changesetBean.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/content/changeset/changesetManager.cfc:212` method 'loadBy' in changesetTagAssignmentBean has no component return type (chain to 'save') |
| 1 | `method 'loadBy' in contentBean has no component return type → core/mura/bean/beanExtendable.cfc` | 14 | low | declares getType() (14 candidates) | `core/modules/v1/cta/configurator.cfm:44` method 'loadBy' in contentBean has no component return type (chain to 'getType') |
| 1 | `method 'loadBy' in contentBean has no component return type → core/mura/bean/beanORMHistorical.cfc` | 2 | low | declares getVersionHistoryIterator() (2 candidates) | `core/mura/client/api/json/v1/jsonApiUtility.cfc:3247` method 'loadBy' in contentBean has no component return type (chain to 'getVersionHistoryIterator') |
| 1 | `method 'loadBy' in oauthClientBean has no component return type → core/mura/json.cfc` | 20 | low | declares validate() (20 candidates) | `admin/core/controllers/cwebservice.cfc:106` method 'loadBy' in oauthClientBean has no component return type (chain to 'validate') |
| 1 | `method 'loadBy' in userDeviceBean has no component return type → core/mura/bean/beanEntity.cfc` | 45 | low | declares save() (45 candidates) | `core/mura/login/loginManager.cfc:308` method 'loadBy' in userDeviceBean has no component return type (chain to 'save') |
| 1 | `method 'loadBy' in userDeviceBean has no component return type → core/mura/user/userBean.cfc` | 2 | low | declares setLastLogin() (2 candidates) | `core/mura/login/loginManager.cfc:308` method 'loadBy' in userDeviceBean has no component return type (chain to 'setLastLogin') |
| 1 | `method 'save' in changesetManager has no component return type → core/mura/content/contentBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/content/changeset/changesetBean.cfc:151` method 'save' in changesetManager has no component return type (chain to 'getAllValues') |
| 1 | `method 'save' in contentManager has no component return type → core/mura/content/contentBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/content/contentBean.cfc:1077` method 'save' in contentManager has no component return type (chain to 'getAllValues') |
| 1 | `method 'save' in emailManager has no component return type → core/mura/configBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/email/emailBean.cfc:180` method 'save' in emailManager has no component return type (chain to 'getAllValues') |
| 1 | `method 'save' in mailinglistManager has no component return type → core/mura/configBean.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/mailinglist/mailinglistBean.cfc:142` method 'save' in mailinglistManager has no component return type (chain to 'getAllValues') |
| 1 | `method 'save' in settingsManager has no component return type → core/mura/settings/settingsBundle.cfc` | 25 | low | declares getAllValues() (25 candidates) | `core/mura/settings/settingsBean.cfc:1272` method 'save' in settingsManager has no component return type (chain to 'getAllValues') |

<details><summary>Groups with several candidates</summary>

- `method 'event' in mura.MuraScope has no component return type → core/mura/event.cfc` — 55 finding(s), 13 candidate(s):
  - medium `core/mura/event.cfc` — declares setValue(); named like the receiver 'event'
  - low `core/mura/cfobject.cfc` — declares setValue()
  - low `core/mura/configBean.cfc` — declares setValue()
  - low `core/mura/servletEvent.cfc` — declares setValue()
  - low `core/mura/bean/bean.cfc` — declares setValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares setValue()
  - low `core/mura/content/contentNavBean.cfc` — declares setValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares setValue()
  - … 5 more
- `method 'getValue' in mura.servletEvent has no component return type → core/mura/content/contentBean.cfc` — 31 finding(s), 15 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getIsNew()
  - low `core/mura/content/contentCommentBean.cfc` — declares getIsNew()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getIsNew()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getIsNew()
  - low `core/mura/content/feed/feedBean.cfc` — declares getIsNew()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getIsNew()
  - low `core/mura/bean/bean.cfc` — declares getIsNew()
  - low `core/mura/category/categoryBean.cfc` — declares getIsNew()
  - … 7 more
- `method 'getBean' in MuraScope has no component return type → core/mura/bean/beanORM.cfc` — 24 finding(s), 15 candidate(s):
  - low `core/mura/bean/beanORM.cfc` — declares loadBy()
  - low `core/mura/category/categoryBean.cfc` — declares loadBy()
  - low `core/mura/content/contentBean.cfc` — declares loadBy()
  - low `core/mura/content/contentCommentBean.cfc` — declares loadBy()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares loadBy()
  - low `core/mura/extend/extendObject.cfc` — declares loadBy()
  - low `core/mura/mailinglist/mailinglistBean.cfc` — declares loadBy()
  - low `core/mura/settings/settingsBean.cfc` — declares loadBy()
  - … 7 more
- `method 'getFeed' in mura.MuraScope has no component return type → core/mura/bean/beanFeed.cfc` — 23 finding(s), 10 candidate(s):
  - low `core/mura/bean/beanFeed.cfc` — declares getIterator()
  - low `core/mura/bean/beanORM.cfc` — declares getIterator()
  - low `core/mura/category/categoryManager.cfc` — declares getIterator()
  - low `core/mura/content/contentManager.cfc` — declares getIterator()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getIterator()
  - low `core/mura/trash/trashManager.cfc` — declares getIterator()
  - low `core/mura/user/userFeedBean.cfc` — declares getIterator()
  - low `core/mura/user/userManager.cfc` — declares getIterator()
  - … 2 more
- `method 'content' in mura.MuraScope has no component return type → core/mura/bean/beanExtendable.cfc` — 16 finding(s), 6 candidate(s):
  - low `core/mura/bean/beanExtendable.cfc` — declares getSubType(), getType()
  - low `core/mura/extend/extendData.cfc` — declares getSubType(), getType()
  - low `core/mura/extend/extendObject.cfc` — declares getSubType(), getType()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getSubType(), getType()
  - low `core/mura/extend/extendSubType.cfc` — declares getSubType(), getType()
  - low `core/mura/user/userBean.cfc` — declares getSubType(), getType()
- `method 'getRBFactory' in settingsBean has no component return type → core/mura/googleAuth.cfc` — 15 finding(s), 2 candidate(s):
  - low `core/mura/googleAuth.cfc` — declares getKey()
  - low `core/mura/resourceBundle/resourceBundleFactory.cfc` — declares getKey()
- `method 'event' in MuraScope has no component return type → core/mura/event.cfc` — 10 finding(s), 13 candidate(s):
  - medium `core/mura/event.cfc` — declares setValue(); named like the receiver 'event'
  - low `core/mura/cfobject.cfc` — declares setValue()
  - low `core/mura/configBean.cfc` — declares setValue()
  - low `core/mura/servletEvent.cfc` — declares setValue()
  - low `core/mura/bean/bean.cfc` — declares setValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares setValue()
  - low `core/mura/content/contentNavBean.cfc` — declares setValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares setValue()
  - … 5 more
- `method 'getApi' in settingsBean has no component return type → core/mura/settings/settingsBean.cfc` — 10 finding(s), 3 candidate(s):
  - low `core/mura/settings/settingsBean.cfc` — declares getEndpoint()
  - low `core/mura/client/api/feed/v1/feedApiUtility.cfc` — declares getEndpoint()
  - low `core/mura/client/api/json/v1/jsonApiUtility.cfc` — declares getEndpoint()
- `method 'getBean' has no component return type → core/mura/bean/bean.cfc` — 10 finding(s), 11 candidate(s):
  - medium `core/mura/bean/bean.cfc` — declares getFeed(); named like the receiver 'Bean'
  - low `core/mura/MasaScope.cfc` — declares getFeed()
  - low `core/mura/cfobject.cfc` — declares getFeed()
  - low `core/mura/bean/beanORM.cfc` — declares getFeed()
  - low `core/mura/iterator/queryIterator.cfc` — declares getFeed()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getFeed()
  - low `core/mura/content/changeset/changesetManager.cfc` — declares getFeed()
  - low `core/mura/content/feed/feedBean.cfc` — declares getFeed()
  - … 3 more
- `method 'loadBy' in fileBean has no component return type → core/mura/content/contentBean.cfc` — 8 finding(s), 45 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - low `core/mura/content/contentManager.cfc` — declares save()
  - low `core/mura/content/contentStatsBean.cfc` — declares save()
  - low `core/mura/content/approval/approvalChainBean.cfc` — declares save()
  - low `core/mura/content/approval/approvalRequestBean.cfc` — declares save()
  - … 37 more
- `method 'getContentRenderer' has no component return type → core/mura/content/contentRenderer.cfc` — 7 finding(s), 2 candidate(s):
  - medium `core/mura/content/contentRenderer.cfc` — declares getCurrentURL(); named like the receiver 'ContentRenderer'
  - low `core/mura/content/contentRendererUtility.cfc` — declares getCurrentURL()
- `method 'getContentBean' has no component return type → core/mura/content/contentNavBean.cfc` — 6 finding(s), 13 candidate(s):
  - low `core/mura/content/contentNavBean.cfc` — declares getValue()
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/event.cfc` — declares getValue()
  - low `core/mura/servletEvent.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue()
  - … 5 more
- `method 'loadBy' in approvalActionBean has no component return type → core/mura/content/approval/approvalChainBean.cfc` — 6 finding(s), 45 candidate(s):
  - low `core/mura/content/approval/approvalChainBean.cfc` — declares save()
  - low `core/mura/content/approval/approvalRequestBean.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - low `core/mura/content/contentManager.cfc` — declares save()
  - low `core/mura/content/contentStatsBean.cfc` — declares save()
  - … 37 more
- `method 'loadBy' in contentBean has no component return type → core/mura/content/contentBean.cfc` — 6 finding(s), 2 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getCrumbArray()
  - low `core/mura/content/contentNavBean.cfc` — declares getCrumbArray()
- `method 'getApi' in settingsBean has no component return type → core/mura/client/api/feed/v1/feedApiUtility.cfc` — 5 finding(s), 3 candidate(s):
  - low `core/mura/client/api/feed/v1/feedApiUtility.cfc` — declares getEndpoint()
  - low `core/mura/client/api/json/v1/jsonApiUtility.cfc` — declares getEndpoint()
  - low `core/mura/settings/settingsBean.cfc` — declares getEndpoint()
- `method 'getBean' has no component return type → core/mura/bean/beanORM.cfc` — 5 finding(s), 15 candidate(s):
  - low `core/mura/bean/beanORM.cfc` — declares loadBy()
  - low `core/mura/category/categoryBean.cfc` — declares loadBy()
  - low `core/mura/content/contentBean.cfc` — declares loadBy()
  - low `core/mura/content/contentCommentBean.cfc` — declares loadBy()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares loadBy()
  - low `core/mura/extend/extendObject.cfc` — declares loadBy()
  - low `core/mura/mailinglist/mailinglistBean.cfc` — declares loadBy()
  - low `core/mura/settings/settingsBean.cfc` — declares loadBy()
  - … 7 more
- `method 'getBean' in mura.MuraScope has no component return type → core/mura/bean/beanORM.cfc` — 5 finding(s), 2 candidate(s):
  - low `core/mura/bean/beanORM.cfc` — declares checkSchema()
  - low `core/mura/client/api/json/v1/jsonApiUtility.cfc` — declares checkSchema()
- `method 'siteConfig' in mura.MuraScope has no component return type → core/modules/v1/filebrowser/model/beans/filebrowser.cfc` — 5 finding(s), 3 candidate(s):
  - low `core/modules/v1/filebrowser/model/beans/filebrowser.cfc` — declares getResourceBundle()
  - low `core/mura/resourceBundle/resourceBundle.cfc` — declares getResourceBundle()
  - low `core/mura/resourceBundle/resourceBundleFactory.cfc` — declares getResourceBundle()
- `method 'event' in MuraScope has no component return type → core/mura/content/contentCommentBean.cfc` — 4 finding(s), 6 candidate(s):
  - low `core/mura/content/contentCommentBean.cfc` — declares getEmail()
  - low `core/mura/content/contentCommenterBean.cfc` — declares getEmail()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getEmail()
  - low `core/mura/mailinglist/memberBean.cfc` — declares getEmail()
  - low `core/mura/user/userBean.cfc` — declares getEmail()
  - low `core/tests/resources/model/beans/widget.cfc` — declares getEmail()
- `method 'getBean' has no component return type → core/mura/content/contentBean.cfc` — 4 finding(s), 9 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares setOrderNo()
  - low `core/mura/extend/extendAttribute.cfc` — declares setOrderNo()
  - low `core/mura/extend/extendRelatedContentSetBean.cfc` — declares setOrderNo()
  - low `core/mura/extend/extendSet.cfc` — declares setOrderNo()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares setOrderNo()
  - low `core/mura/formBuilder/fieldBean.cfc` — declares setOrderNo()
  - low `core/mura/formBuilder/fieldOptionBean.cfc` — declares setOrderNo()
  - low `core/mura/settings/settingsBean.cfc` — declares setOrderNo()
  - … 1 more
- `method 'getCacheFactory' in settingsBean has no component return type → core/mura/cache/cacheAbstract.cfc` — 4 finding(s), 4 candidate(s):
  - low `core/mura/cache/cacheAbstract.cfc` — declares purgeAll()
  - low `core/mura/cache/cacheAdvanced.cfc` — declares purgeAll()
  - low `core/mura/cache/provider/cacheAdobe.cfc` — declares purgeAll()
  - low `core/mura/cache/provider/cacheLucee.cfc` — declares purgeAll()
- `method 'loadBy' in extendRelatedContentSetBean has no component return type → core/mura/bean/beanRemotePointer.cfc` — 4 finding(s), 2 candidate(s):
  - low `core/mura/bean/beanRemotePointer.cfc` — declares getEntityType()
  - low `core/mura/extend/extendRelatedContentSetBean.cfc` — declares getEntityType()
- `method 'getBean' in MuraScope has no component return type → core/mura/bean/beanEntity.cfc` — 3 finding(s), 45 candidate(s):
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - … 37 more
- `method 'getContentBean' has no component return type → core/mura/cfobject.cfc` — 3 finding(s), 7 candidate(s):
  - low `core/mura/cfobject.cfc` — declares valueExists()
  - low `core/mura/event.cfc` — declares valueExists()
  - low `core/mura/servletEvent.cfc` — declares valueExists()
  - low `core/mura/bean/bean.cfc` — declares valueExists()
  - low `core/mura/plugin/pluginApplication.cfc` — declares valueExists()
  - low `core/mura/settings/settingsBundle.cfc` — declares valueExists()
  - low `core/mura/settings/settingsBundleBean.cfc` — declares valueExists()
- `method 'getContentBean' has no component return type → core/mura/content/contentBean.cfc` — 3 finding(s), 9 candidate(s):
  - medium `core/mura/content/contentBean.cfc` — declares getParent(); named like the receiver 'ContentBean'
  - low `core/mura/MasaScope.cfc` — declares getParent()
  - low `core/mura/bean/beanFactory.cfc` — declares getParent()
  - low `core/mura/cache/cacheAbstract.cfc` — declares getParent()
  - low `core/mura/category/categoryBean.cfc` — declares getParent()
  - low `core/mura/content/contentCategoryAssignBean.cfc` — declares getParent()
  - low `core/mura/content/contentCommentBean.cfc` — declares getParent()
  - low `core/mura/content/contentNavBean.cfc` — declares getParent()
  - … 1 more
- `method 'getEntity' has no component return type → core/mura/bean/bean.cfc` — 3 finding(s), 2 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares getDbUtility()
  - low `core/mura/bean/beanORM.cfc` — declares getDbUtility()
- `method 'getEntity' has no component return type → core/mura/bean/beanORM.cfc` — 3 finding(s), 2 candidate(s):
  - low `core/mura/bean/beanORM.cfc` — declares getLoadSQLColumnsAndTables(), getPrimaryKey()
  - low `core/mura/content/contentCategoryAssignBean.cfc` — declares getLoadSQLColumnsAndTables(), getPrimaryKey()
- `method 'getFeed' has no component return type → core/mura/user/userFeedBean.cfc` — 3 finding(s), 10 candidate(s):
  - low `core/mura/user/userFeedBean.cfc` — declares getIterator()
  - low `core/mura/user/userManager.cfc` — declares getIterator()
  - low `core/mura/bean/beanFeed.cfc` — declares getIterator()
  - low `core/mura/bean/beanORM.cfc` — declares getIterator()
  - low `core/mura/category/categoryManager.cfc` — declares getIterator()
  - low `core/mura/content/contentManager.cfc` — declares getIterator()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getIterator()
  - low `core/mura/trash/trashManager.cfc` — declares getIterator()
  - … 2 more
- `method 'loadBy' in approvalActionBean has no component return type → core/mura/content/contentCommentBean.cfc` — 3 finding(s), 4 candidate(s):
  - low `core/mura/content/contentCommentBean.cfc` — declares setUserID()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares setUserID()
  - low `core/mura/content/rater/rateBean.cfc` — declares setUserID()
  - low `core/mura/user/userBean.cfc` — declares setUserID()
- `method 'loadBy' in fileBean has no component return type → core/mura/content/contentFileMetaDataBean.cfc` — 3 finding(s), 2 candidate(s):
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares setAltText()
  - low `core/mura/content/file/fileBean.cfc` — declares setAltText()
- `method 'content' in MuraScope has no component return type → core/mura/content/contentBean.cfc` — 2 finding(s), 15 candidate(s):
  - medium `core/mura/content/contentBean.cfc` — declares getIsNew(); named like the receiver 'content'
  - low `core/mura/bean/bean.cfc` — declares getIsNew()
  - low `core/mura/category/categoryBean.cfc` — declares getIsNew()
  - low `core/mura/content/contentCommentBean.cfc` — declares getIsNew()
  - low `core/mura/extend/extendAttribute.cfc` — declares getIsNew()
  - low `core/mura/extend/extendSet.cfc` — declares getIsNew()
  - low `core/mura/extend/extendSubType.cfc` — declares getIsNew()
  - low `core/mura/mailinglist/mailinglistBean.cfc` — declares getIsNew()
  - … 7 more
- `method 'event' has no component return type → core/mura/content/contentBean.cfc` — 2 finding(s), 4 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getContentID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getContentID()
  - low `core/mura/content/rater/rateBean.cfc` — declares getContentID()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getContentID()
- `method 'event' has no component return type → core/mura/event.cfc` — 2 finding(s), 13 candidate(s):
  - medium `core/mura/event.cfc` — declares getValue(); named like the receiver 'event'
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/servletEvent.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - low `core/mura/content/contentNavBean.cfc` — declares getValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares getValue()
  - … 5 more
- `method 'getActiveContent' in contentManager has no component return type → core/mura/content/contentBean.cfc` — 2 finding(s), 6 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getURL()
  - low `core/mura/content/contentCommentBean.cfc` — declares getURL()
  - low `core/mura/content/contentManager.cfc` — declares getURL()
  - low `core/mura/content/contentNavBean.cfc` — declares getURL()
  - low `core/mura/user/userRedirectBean.cfc` — declares getURL()
  - low `core/mura/content/file/fileBean.cfc` — declares getURL()
- `method 'getClassExtensionManager' in configBean has no component return type → core/mura/bean/beanExtendable.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/bean/beanExtendable.cfc` — declares getExtendedData()
  - low `core/mura/extend/extendManager.cfc` — declares getExtendedData()
- `method 'getDefaultBeanFactory' has no component return type → core/mura/bean/beanFactory.cfc` — 2 finding(s), 3 candidate(s):
  - low `core/mura/bean/beanFactory.cfc` — declares containsBean(), getBean()
  - low `core/mura/bean/ioc.cfc` — declares containsBean(), getBean()
  - low `core/mura/plugin/pluginApplication.cfc` — declares containsBean(), getBean()
- `method 'getDisplayInterval' in contentBean has no component return type → core/mura/content/contentBean.cfc` — 2 finding(s), 25 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentNavBean.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - … 17 more
- `method 'getEvent' has no component return type → core/mura/event.cfc` — 2 finding(s), 13 candidate(s):
  - medium `core/mura/event.cfc` — declares setValue(), getValue(); named like the receiver 'Event'
  - low `core/mura/cfobject.cfc` — declares setValue(), getValue()
  - low `core/mura/configBean.cfc` — declares setValue(), getValue()
  - low `core/mura/servletEvent.cfc` — declares setValue(), getValue()
  - low `core/mura/bean/bean.cfc` — declares setValue(), getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares setValue(), getValue()
  - low `core/mura/content/contentNavBean.cfc` — declares setValue(), getValue()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares setValue(), getValue()
  - … 5 more
- `method 'getFeed' in mura.MuraScope has no component return type → core/mura/content/feed/feedBean.cfc` — 2 finding(s), 4 candidate(s):
  - medium `core/mura/content/feed/feedBean.cfc` — declares setIsPublic(); named like the receiver 'Feed'
  - low `core/mura/mailinglist/mailinglistBean.cfc` — declares setIsPublic()
  - low `core/mura/user/userBean.cfc` — declares setIsPublic()
  - low `core/mura/user/userFeedBean.cfc` — declares setIsPublic()
- `method 'getLocaleUtils' has no component return type → core/mura/settings/settingsBean.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/settings/settingsBean.cfc` — declares getJSDateKey()
  - low `core/mura/resourceBundle/utils.cfc` — declares getJSDateKey()
- `method 'getRazunaSettings' in settingsBean has no component return type → core/mura/settings/settingsBean.cfc` — 2 finding(s), 45 candidate(s):
  - low `core/mura/settings/settingsBean.cfc` — declares save()
  - low `core/mura/settings/settingsImageSizeBean.cfc` — declares save()
  - low `core/mura/settings/settingsManager.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - … 37 more
- `method 'getRelatedFeed' has no component return type → core/mura/bean/beanFeed.cfc` — 2 finding(s), 15 candidate(s):
  - low `core/mura/bean/beanFeed.cfc` — declares getQuery()
  - low `core/mura/content/contentCommentBean.cfc` — declares getQuery()
  - low `core/mura/content/contentStatsBean.cfc` — declares getQuery()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getQuery()
  - low `core/mura/iterator/queryIterator.cfc` — declares getQuery()
  - low `core/mura/plugin/pluginDisplayObjectBean.cfc` — declares getQuery()
  - low `core/mura/plugin/pluginScriptBean.cfc` — declares getQuery()
  - low `core/mura/settings/settingsImageSizeBean.cfc` — declares getQuery()
  - … 7 more
- `method 'getSite' has no component return type → core/mura/googleAuth.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/googleAuth.cfc` — declares getKey()
  - low `core/mura/resourceBundle/resourceBundleFactory.cfc` — declares getKey()
- `method 'getSite' in mura.servletEvent has no component return type → core/mura/configBean.cfc` — 2 finding(s), 2 candidate(s):
  - low `core/mura/configBean.cfc` — declares getAssetPath()
  - low `core/mura/settings/settingsBean.cfc` — declares getAssetPath()
- `method 'getSubsystemBeanFactory' has no component return type → core/mura/bean/beanFactory.cfc` — 2 finding(s), 3 candidate(s):
  - low `core/mura/bean/beanFactory.cfc` — declares containsBean(), getBean()
  - low `core/mura/bean/ioc.cfc` — declares containsBean(), getBean()
  - low `core/mura/plugin/pluginApplication.cfc` — declares containsBean(), getBean()
- `method 'getUserBean' has no component return type → core/mura/user/sessionUserFacade.cfc` — 2 finding(s), 13 candidate(s):
  - low `core/mura/user/sessionUserFacade.cfc` — declares getValue()
  - low `core/mura/cfobject.cfc` — declares getValue()
  - low `core/mura/configBean.cfc` — declares getValue()
  - low `core/mura/event.cfc` — declares getValue()
  - low `core/mura/servletEvent.cfc` — declares getValue()
  - low `core/mura/bean/bean.cfc` — declares getValue()
  - low `core/mura/bean/beanExtendable.cfc` — declares getValue()
  - low `core/mura/content/contentNavBean.cfc` — declares getValue()
  - … 5 more
- `method 'loadBy' in categoryBean has no component return type → core/mura/content/changeset/changesetBean.cfc` — 2 finding(s), 6 candidate(s):
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getCategoryID()
  - low `core/mura/content/feed/feedBean.cfc` — declares getCategoryID()
  - low `core/mura/category/categoryBean.cfc` — declares getCategoryID()
  - low `core/mura/extend/extendSet.cfc` — declares getCategoryID()
  - low `core/mura/user/userBean.cfc` — declares getCategoryID()
  - low `core/mura/user/userFeedBean.cfc` — declares getCategoryID()
- `method 'loadBy' in contentBean has no component return type → core/mura/bean/bean.cfc` — 2 finding(s), 4 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares exists()
  - low `core/mura/extend/extendAttribute.cfc` — declares exists()
  - low `core/mura/extend/extendSet.cfc` — declares exists()
  - low `core/mura/extend/extendSubType.cfc` — declares exists()
- `method 'loadBy' in contentBean has no component return type → core/mura/configBean.cfc` — 2 finding(s), 25 candidate(s):
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - … 17 more
- `method 'loadBy' in contentFilenameArchiveBean has no component return type → core/mura/content/contentBean.cfc` — 2 finding(s), 45 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - low `core/mura/content/contentManager.cfc` — declares save()
  - low `core/mura/content/contentStatsBean.cfc` — declares save()
  - low `core/mura/content/approval/approvalChainBean.cfc` — declares save()
  - low `core/mura/content/approval/approvalRequestBean.cfc` — declares save()
  - … 37 more
- `method 'content' has no component return type → core/mura/content/contentBean.cfc` — 1 finding(s), 4 candidate(s):
  - medium `core/mura/content/contentBean.cfc` — declares getCrumbIterator(); named like the receiver 'content'
  - low `core/mura/category/categoryBean.cfc` — declares getCrumbIterator()
  - low `core/mura/content/contentCommentBean.cfc` — declares getCrumbIterator()
  - low `core/mura/content/contentNavBean.cfc` — declares getCrumbIterator()
- `method 'getApprovalRequest' in contentBean has no component return type → core/mura/content/approval/approvalRequestBean.cfc` — 1 finding(s), 2 candidate(s):
  - medium `core/mura/content/approval/approvalRequestBean.cfc` — declares setStatus(); named like the receiver 'ApprovalRequest'
  - low `core/mura/email/emailBean.cfc` — declares setStatus()
- `method 'getBean' has no component return type → core/mura/bean/beanEntity.cfc` — 1 finding(s), 45 candidate(s):
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - … 37 more
- `method 'getBean' has no component return type → core/mura/client/api/json/v1/jsonApiUtility.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/client/api/json/v1/jsonApiUtility.cfc` — declares checkSchema()
  - low `core/mura/bean/beanORM.cfc` — declares checkSchema()
- `method 'getCache' has no component return type → core/mura/cache/cacheAbstract.cfc` — 1 finding(s), 4 candidate(s):
  - low `core/mura/cache/cacheAbstract.cfc` — declares purge()
  - low `core/mura/cache/cacheAdvanced.cfc` — declares purge()
  - low `core/mura/cache/provider/cacheAdobe.cfc` — declares purge()
  - low `core/mura/cache/provider/cacheLucee.cfc` — declares purge()
- `method 'getCacheFactory' has no component return type → core/mura/cache/cacheAbstract.cfc` — 1 finding(s), 4 candidate(s):
  - low `core/mura/cache/cacheAbstract.cfc` — declares purgeAll()
  - low `core/mura/cache/cacheAdvanced.cfc` — declares purgeAll()
  - low `core/mura/cache/provider/cacheAdobe.cfc` — declares purgeAll()
  - low `core/mura/cache/provider/cacheLucee.cfc` — declares purgeAll()
- `method 'getClassExtensionManager' in configBean has no component return type → core/mura/extend/extendData.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/extend/extendData.cfc` — declares getDefinitionsQuery()
  - low `core/mura/extend/extendManager.cfc` — declares getDefinitionsQuery()
- `method 'getConfig' has no component return type → core/mura/plugin/pluginConfig.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/plugin/pluginConfig.cfc` — declares getAssignedSites()
  - low `core/mura/plugin/pluginManager.cfc` — declares getAssignedSites()
- `method 'getConfig' has no component return type → core/mura/plugin/pluginManager.cfc` — 1 finding(s), 3 candidate(s):
  - low `core/mura/plugin/pluginManager.cfc` — declares discoverBeans()
  - low `core/mura/bean/ioc.cfc` — declares discoverBeans()
  - low `core/mura/settings/settingsBean.cfc` — declares discoverBeans()
- `method 'getContentBean' in mura.servletEvent has no component return type → core/mura/content/contentBean.cfc` — 1 finding(s), 4 candidate(s):
  - medium `core/mura/content/contentBean.cfc` — declares getContentID(); named like the receiver 'ContentBean'
  - low `core/mura/content/feed/feedBean.cfc` — declares getContentID()
  - low `core/mura/content/rater/rateBean.cfc` — declares getContentID()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getContentID()
- `method 'getDisplayInterval' in contentBean has no component return type → core/mura/configBean.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - … 17 more
- `method 'getExtendedData' has no component return type → core/mura/bean/bean.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - … 17 more
- `method 'getExtendedData' in contentBean has no component return type → core/mura/content/contentBean.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentNavBean.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - … 17 more
- `method 'getFeed' in categoryBean has no component return type → core/mura/content/contentCommentFeedBean.cfc` — 1 finding(s), 26 candidate(s):
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares setSiteID()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares setSiteID()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares setSiteID()
  - low `core/mura/content/dataCollection/dataCollectionBean.cfc` — declares setSiteID()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares setSiteID()
  - low `core/mura/content/rater/rateBean.cfc` — declares setSiteID()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares setSiteID()
  - low `core/mura/bean/beanExtendable.cfc` — declares setSiteID()
  - … 18 more
- `method 'getFeed' in categoryBean has no component return type → core/mura/content/contentManager.cfc` — 1 finding(s), 10 candidate(s):
  - low `core/mura/content/contentManager.cfc` — declares getIterator()
  - low `core/mura/content/changeset/changesetManager.cfc` — declares getIterator()
  - low `core/mura/content/feed/feedBean.cfc` — declares getIterator()
  - low `core/mura/bean/beanFeed.cfc` — declares getIterator()
  - low `core/mura/bean/beanORM.cfc` — declares getIterator()
  - low `core/mura/category/categoryManager.cfc` — declares getIterator()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getIterator()
  - low `core/mura/trash/trashManager.cfc` — declares getIterator()
  - … 2 more
- `method 'getFeed' in categoryBean has no component return type → core/mura/content/feed/feedBean.cfc` — 1 finding(s), 15 candidate(s):
  - low `core/mura/content/feed/feedBean.cfc` — declares getQuery()
  - low `core/mura/content/contentCommentBean.cfc` — declares getQuery()
  - low `core/mura/content/contentStatsBean.cfc` — declares getQuery()
  - low `core/mura/content/changeset/changesetManager.cfc` — declares getQuery()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getQuery()
  - low `core/mura/content/rater/rateBean.cfc` — declares getQuery()
  - low `core/mura/bean/beanFeed.cfc` — declares getQuery()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getQuery()
  - … 7 more
- `method 'getFeed' in mura.MuraScope has no component return type → core/mura/bean/beanExtendable.cfc` — 1 finding(s), 26 candidate(s):
  - low `core/mura/bean/beanExtendable.cfc` — declares setSiteID()
  - low `core/mura/bean/beanFeed.cfc` — declares setSiteID()
  - low `core/mura/category/categoryFeedBean.cfc` — declares setSiteID()
  - low `core/mura/content/contentCommentFeedBean.cfc` — declares setSiteID()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares setSiteID()
  - low `core/mura/email/emailBean.cfc` — declares setSiteID()
  - low `core/mura/extend/extendAttribute.cfc` — declares setSiteID()
  - low `core/mura/extend/extendData.cfc` — declares setSiteID()
  - … 18 more
- `method 'getKidsIterator' has no component return type → core/mura/content/contentCommentBean.cfc` — 1 finding(s), 15 candidate(s):
  - low `core/mura/content/contentCommentBean.cfc` — declares getQuery()
  - low `core/mura/content/contentStatsBean.cfc` — declares getQuery()
  - low `core/mura/content/changeset/changesetManager.cfc` — declares getQuery()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getQuery()
  - low `core/mura/content/feed/feedBean.cfc` — declares getQuery()
  - low `core/mura/content/rater/rateBean.cfc` — declares getQuery()
  - low `core/mura/bean/beanFeed.cfc` — declares getQuery()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getQuery()
  - … 7 more
- `method 'getManager' has no component return type → core/mura/extend/extendAttribute.cfc` — 1 finding(s), 45 candidate(s):
  - low `core/mura/extend/extendAttribute.cfc` — declares save()
  - low `core/mura/extend/extendObject.cfc` — declares save()
  - low `core/mura/extend/extendSet.cfc` — declares save()
  - low `core/mura/extend/extendSubType.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - … 37 more
- `method 'getObject' in trashItemBean has no component return type → core/mura/bean/beanEntity.cfc` — 1 finding(s), 45 candidate(s):
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - … 37 more
- `method 'getParent' in contentBean has no component return type → core/mura/bean/beanExtendable.cfc` — 1 finding(s), 14 candidate(s):
  - low `core/mura/bean/beanExtendable.cfc` — declares getType()
  - low `core/mura/content/contentBean.cfc` — declares getType()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares getType()
  - low `core/mura/extend/extendAttribute.cfc` — declares getType()
  - low `core/mura/extend/extendData.cfc` — declares getType()
  - low `core/mura/extend/extendObject.cfc` — declares getType()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getType()
  - low `core/mura/extend/extendSubType.cfc` — declares getType()
  - … 6 more
- `method 'getParent' in mura.MuraScope has no component return type → core/mura/content/contentBean.cfc` — 1 finding(s), 14 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getType()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares getType()
  - low `core/mura/content/favorite/favoriteBean.cfc` — declares getType()
  - low `core/mura/content/feed/feedBean.cfc` — declares getType()
  - low `core/mura/bean/beanExtendable.cfc` — declares getType()
  - low `core/mura/extend/extendAttribute.cfc` — declares getType()
  - low `core/mura/extend/extendData.cfc` — declares getType()
  - low `core/mura/extend/extendObject.cfc` — declares getType()
  - … 6 more
- `method 'getRBFactory' has no component return type → core/mura/resourceBundle/resourceBundle.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/resourceBundle/resourceBundle.cfc` — declares getUtils()
  - low `core/mura/resourceBundle/resourceBundleFactory.cfc` — declares getUtils()
- `method 'getUserBean' has no component return type → core/mura/user/userBean.cfc` — 1 finding(s), 25 candidate(s):
  - medium `core/mura/user/userBean.cfc` — declares getAllValues(); named like the receiver 'UserBean'
  - low `core/mura/user/sessionUserFacade.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - … 17 more
- `method 'getcontentRenderer' in settingsBean has no component return type → core/mura/content/contentRenderer.cfc` — 1 finding(s), 2 candidate(s):
  - medium `core/mura/content/contentRenderer.cfc` — declares getURLStem(); named like the receiver 'contentRenderer'
  - low `core/mura/content/contentServer.cfc` — declares getURLStem()
- `method 'loadBy' in approvalChainAssignmentBean has no component return type → admin/core/controllers/cchain.cfc` — 1 finding(s), 45 candidate(s):
  - low `admin/core/controllers/cchain.cfc` — declares save()
  - low `admin/core/controllers/cchangesets.cfc` — declares save()
  - low `admin/core/controllers/cwebservice.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - … 37 more
- `method 'loadBy' in approvalChainBean has no component return type → admin/core/controllers/cchain.cfc` — 1 finding(s), 45 candidate(s):
  - low `admin/core/controllers/cchain.cfc` — declares save()
  - low `admin/core/controllers/cchangesets.cfc` — declares save()
  - low `admin/core/controllers/cwebservice.cfc` — declares save()
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - … 37 more
- `method 'loadBy' in approvalChainMembershipBean has no component return type → core/mura/content/approval/approvalChainBean.cfc` — 1 finding(s), 45 candidate(s):
  - low `core/mura/content/approval/approvalChainBean.cfc` — declares save()
  - low `core/mura/content/approval/approvalRequestBean.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - low `core/mura/content/contentManager.cfc` — declares save()
  - low `core/mura/content/contentStatsBean.cfc` — declares save()
  - … 37 more
- `method 'loadBy' in approvalChainMembershipBean has no component return type → core/mura/content/approval/approvalChainMembershipBean.cfc` — 1 finding(s), 9 candidate(s):
  - low `core/mura/content/approval/approvalChainMembershipBean.cfc` — declares setOrderNo()
  - low `core/mura/content/contentBean.cfc` — declares setOrderNo()
  - low `core/mura/extend/extendAttribute.cfc` — declares setOrderNo()
  - low `core/mura/extend/extendRelatedContentSetBean.cfc` — declares setOrderNo()
  - low `core/mura/extend/extendSet.cfc` — declares setOrderNo()
  - low `core/mura/formBuilder/datarecordBean.cfc` — declares setOrderNo()
  - low `core/mura/formBuilder/fieldBean.cfc` — declares setOrderNo()
  - low `core/mura/formBuilder/fieldOptionBean.cfc` — declares setOrderNo()
  - … 1 more
- `method 'loadBy' in approvalRequestBean has no component return type → core/mura/content/changeset/changesetBean.cfc` — 1 finding(s), 45 candidate(s):
  - low `core/mura/content/changeset/changesetBean.cfc` — declares save()
  - low `core/mura/content/changeset/changesetManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - low `core/mura/content/contentManager.cfc` — declares save()
  - low `core/mura/content/contentStatsBean.cfc` — declares save()
  - … 37 more
- `method 'loadBy' in beanEntity has no component return type → core/mura/extend/extendRelatedContentSetBean.cfc` — 1 finding(s), 3 candidate(s):
  - low `core/mura/extend/extendRelatedContentSetBean.cfc` — declares getDisplayName()
  - low `core/mura/bean/beanEntity.cfc` — declares getDisplayName()
  - low `core/mura/content/feed/feedBean.cfc` — declares getDisplayName()
- `method 'loadBy' in categoryBean has no component return type → core/mura/content/feed/feedBean.cfc` — 1 finding(s), 6 candidate(s):
  - low `core/mura/content/feed/feedBean.cfc` — declares getCategoryID()
  - low `core/mura/content/changeset/changesetBean.cfc` — declares getCategoryID()
  - low `core/mura/category/categoryBean.cfc` — declares getCategoryID()
  - low `core/mura/extend/extendSet.cfc` — declares getCategoryID()
  - low `core/mura/user/userBean.cfc` — declares getCategoryID()
  - low `core/mura/user/userFeedBean.cfc` — declares getCategoryID()
- `method 'loadBy' in changesetCategoryAssignmentBean has no component return type → core/mura/content/changeset/changesetBean.cfc` — 1 finding(s), 45 candidate(s):
  - low `core/mura/content/changeset/changesetBean.cfc` — declares save()
  - low `core/mura/content/changeset/changesetManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - low `core/mura/content/contentManager.cfc` — declares save()
  - low `core/mura/content/contentStatsBean.cfc` — declares save()
  - … 37 more
- `method 'loadBy' in changesetRollBackBean has no component return type → core/mura/content/changeset/changesetBean.cfc` — 1 finding(s), 45 candidate(s):
  - low `core/mura/content/changeset/changesetBean.cfc` — declares save()
  - low `core/mura/content/changeset/changesetManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - low `core/mura/content/contentManager.cfc` — declares save()
  - low `core/mura/content/contentStatsBean.cfc` — declares save()
  - … 37 more
- `method 'loadBy' in changesetTagAssignmentBean has no component return type → core/mura/content/changeset/changesetBean.cfc` — 1 finding(s), 45 candidate(s):
  - low `core/mura/content/changeset/changesetBean.cfc` — declares save()
  - low `core/mura/content/changeset/changesetManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - low `core/mura/content/contentManager.cfc` — declares save()
  - low `core/mura/content/contentStatsBean.cfc` — declares save()
  - … 37 more
- `method 'loadBy' in contentBean has no component return type → core/mura/bean/beanExtendable.cfc` — 1 finding(s), 14 candidate(s):
  - low `core/mura/bean/beanExtendable.cfc` — declares getType()
  - low `core/mura/content/contentBean.cfc` — declares getType()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares getType()
  - low `core/mura/extend/extendAttribute.cfc` — declares getType()
  - low `core/mura/extend/extendData.cfc` — declares getType()
  - low `core/mura/extend/extendObject.cfc` — declares getType()
  - low `core/mura/extend/extendObjectFeedBean.cfc` — declares getType()
  - low `core/mura/extend/extendSubType.cfc` — declares getType()
  - … 6 more
- `method 'loadBy' in contentBean has no component return type → core/mura/bean/beanORMHistorical.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/bean/beanORMHistorical.cfc` — declares getVersionHistoryIterator()
  - low `core/mura/content/contentBean.cfc` — declares getVersionHistoryIterator()
- `method 'loadBy' in oauthClientBean has no component return type → core/mura/json.cfc` — 1 finding(s), 20 candidate(s):
  - low `core/mura/json.cfc` — declares validate()
  - low `core/mura/queryParam.cfc` — declares validate()
  - low `core/mura/bean/bean.cfc` — declares validate()
  - low `core/mura/bean/beanORM.cfc` — declares validate()
  - low `core/mura/bean/beanValidator.cfc` — declares validate()
  - low `core/mura/content/contentBean.cfc` — declares validate()
  - low `core/mura/content/contentManager.cfc` — declares validate()
  - low `core/mura/extend/extendAttribute.cfc` — declares validate()
  - … 12 more
- `method 'loadBy' in userDeviceBean has no component return type → core/mura/bean/beanEntity.cfc` — 1 finding(s), 45 candidate(s):
  - low `core/mura/bean/beanEntity.cfc` — declares save()
  - low `core/mura/bean/beanORM.cfc` — declares save()
  - low `core/mura/category/categoryBean.cfc` — declares save()
  - low `core/mura/category/categoryManager.cfc` — declares save()
  - low `core/mura/content/contentBean.cfc` — declares save()
  - low `core/mura/content/contentCommentBean.cfc` — declares save()
  - low `core/mura/content/contentDisplayIntervalBean.cfc` — declares save()
  - low `core/mura/content/contentFileMetaDataBean.cfc` — declares save()
  - … 37 more
- `method 'loadBy' in userDeviceBean has no component return type → core/mura/user/userBean.cfc` — 1 finding(s), 2 candidate(s):
  - low `core/mura/user/userBean.cfc` — declares setLastLogin()
  - low `core/mura/user/userDeviceBean.cfc` — declares setLastLogin()
- `method 'save' in changesetManager has no component return type → core/mura/content/contentBean.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentNavBean.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - … 17 more
- `method 'save' in contentManager has no component return type → core/mura/content/contentBean.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentNavBean.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/content/reminder/reminderBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - … 17 more
- `method 'save' in emailManager has no component return type → core/mura/configBean.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - … 17 more
- `method 'save' in mailinglistManager has no component return type → core/mura/configBean.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - low `core/mura/content/contentBean.cfc` — declares getAllValues()
  - low `core/mura/content/contentCommentBean.cfc` — declares getAllValues()
  - … 17 more
- `method 'save' in settingsManager has no component return type → core/mura/settings/settingsBundle.cfc` — 1 finding(s), 25 candidate(s):
  - low `core/mura/settings/settingsBundle.cfc` — declares getAllValues()
  - low `core/mura/settings/settingsBundleBean.cfc` — declares getAllValues()
  - low `core/mura/configBean.cfc` — declares getAllValues()
  - low `core/mura/event.cfc` — declares getAllValues()
  - low `core/mura/queryParam.cfc` — declares getAllValues()
  - low `core/mura/servletEvent.cfc` — declares getAllValues()
  - low `core/mura/bean/bean.cfc` — declares getAllValues()
  - low `core/mura/bean/beanExtendable.cfc` — declares getAllValues()
  - … 17 more

</details>

## Method definitions — a method not found where it was looked for

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 15 | `parsexml` | 2 | low | declares parseXML (2 candidates) | `admin/core/controllers/cextend.cfc:110` not found in extends chain |
| 10 | `getsite` | 34 | low | declares getSite (34 candidates) | `core/modules/v1/component/index.cfm:123` no qualifier, not in file |
| 8 | `showitemmeta` | 1 | medium | declares showItemMeta | `core/modules/v1/collection/includes/dsp_content_list.cfm:285` no qualifier, not in file |
| 5 | `geturlstem` | 2 | low | declares getURLStem (2 candidates) | `core/modules/v1/category_summary/index.cfm:100` no qualifier, not in file |
| 4 | `dspobject` | 2 | low | declares dspObject (2 candidates) | `core/modules/v1/calendar/index.cfm:124` no qualifier, not in file |
| 4 | `event` | 1 | medium | declares event | `core/mura/plugin/pluginManager.cfc:1422` method 'event' not found in mura.event |
| 4 | `getpersonalizationid` | 2 | low | declares getPersonalizationID (2 candidates) | `core/modules/v1/favorites/index.cfm:16` no qualifier, not in file |
| 3 | `dspfoldernav` | 1 | medium | declares dspFolderNav | `core/modules/v1/nav/dsp_folder.cfm:78` no qualifier, not in file |
| 3 | `getshowinlineeditor` | 1 | medium | declares getShowInlineEditor | `admin/core/utilities/modal/toolbar.cfm:100` no qualifier, not in file |
| 2 | `dspobject_include` | 1 | medium | declares dspObject_Include | `core/modules/v1/collection/index.cfm:283` no qualifier, not in file |
| 2 | `getbean` | 21 | low | declares getBean (20 candidates) | `core/appcfc/onRequestStart_include.cfm:163` no qualifier, not in file |
| 2 | `getcurrenturl` | 2 | low | declares getCurrentURL (2 candidates) | `core/modules/v1/comments/index.cfm:404` no qualifier, not in file |
| 2 | `getformservice` | 0 | none |  | `core/mura/formBuilder/formBean.cfc:249` not found in extends chain |
| 2 | `getservicefactory` | 2 | low | declares getServiceFactory (2 candidates) | `core/appcfc/onApplicationStart_include.cfm:486` no qualifier, not in file |
| 2 | `uselayoutmanager` | 1 | medium | declares useLayoutManager | `admin/core/utilities/modal/toolbar.cfm:382` no qualifier, not in file |
| 1 | `buildindexmetatdata` | 0 | none |  | `core/mura/dbUtility.cfc:1278` not found in extends chain |
| 1 | `callwithstructargs` | 0 | none |  | `core/mura/client/api/soap/v1/muraProxy.cfc:275` not found in extends chain |
| 1 | `createcssid` | 2 | low | declares createCSSID (2 candidates) | `core/modules/v1/mailing_list/index.cfm:101` no qualifier, not in file |
| 1 | `createhref` | 4 | low | declares createHREF (4 candidates) | `core/modules/v1/favorites/index.cfm:29` no qualifier, not in file |
| 1 | `dspnestednav` | 1 | medium | declares dspNestedNav | `core/modules/v1/nav/dsp_multilevel.cfm:78` no qualifier, not in file |
| 1 | `dsppeernav` | 1 | medium | declares dspPeerNav | `core/modules/v1/nav/dsp_peer.cfm:76` no qualifier, not in file |
| 1 | `dspstandardnav` | 1 | medium | declares dspStandardNav | `core/modules/v1/nav/dsp_standard.cfm:78` no qualifier, not in file |
| 1 | `dspsubnav` | 1 | medium | declares dspSubNav | `core/modules/v1/nav/dsp_sub.cfm:76` no qualifier, not in file |
| 1 | `dsptopnav` | 0 | none |  | `core/modules/v1/nav/dsp_top.cfm:78` no qualifier, not in file |
| 1 | `generateeditablehook` | 2 | low | declares generateEditableHook (2 candidates) | `admin/core/utilities/modal/toolbar.cfm:157` no qualifier, not in file |
| 1 | `getallvalues` | 25 | low | declares getAllValues; beside the calling file (25 candidates) | `core/mura/cfobject.cfc:213` no qualifier, not in file |
| 1 | `getclassfullname` | 0 | none |  | `core/mura/cfobject.cfc:267` no qualifier, not in file |
| 1 | `getcurrentuser` | 1 | medium | declares getCurrentUser | `core/modules/v1/gotofirstchild/index.cfm:86` no qualifier, not in file |
| 1 | `getdbtype` | 3 | low | declares getDbType; beside the calling file (3 candidates) | `core/mura/bean/beanORM.cfc:221` method 'getDbType' not found in dbUtility |
| 1 | `getfieldservice` | 0 | none |  | `core/mura/formBuilder/fieldBean.cfc:443` not found in extends chain |
| 1 | `getreportdata` | 1 | medium | declares getReportData; beside the calling file | `core/mura/content/contentManager.cfc:2021` method 'getReportData' not found in contentUtility |
| 1 | `getsessionsearch` | 2 | low | declares getSessionSearch; beside the calling file (2 candidates) | `core/mura/dashboard/dashboardManager.cfc:171` method 'getSessionSearch' not found in emailGateway |
| 1 | `getshowtoolbar` | 1 | medium | declares getShowToolbar | `admin/core/utilities/modal/toolbar.cfm:153` no qualifier, not in file |
| 1 | `hasdiscriminatorvalue` | 0 | none |  | `core/mura/bean/beanFeed.cfc:839` not found in extends chain |
| 1 | `hasvalue` | 0 | none |  | `core/mura/publisherKeys.cfc:99` method 'hasValue' not found in mura.cfobject |
| 1 | `logerror` | 2 | low | declares logError (2 candidates) | `core/mura/client/api/feed/v1/feedApiUtility.cfc:174` not found in extends chain |
| 1 | `newresultquery` | 2 | low | declares newResultQuery (2 candidates) | `core/modules/v1/search/index.cfm:115` no qualifier, not in file |
| 1 | `pathformat` | 2 | low | declares PathFormat; beside the calling file (2 candidates) | `core/mura/utility.cfc:867` not found in extends chain |
| 1 | `readaddress` | 3 | low | declares readAddress (3 candidates) | `core/mura/client/api/soap/v1/user.cfc:114` not found in parent component |
| 1 | `readbyemail` | 0 | none |  | `core/mura/client/api/soap/v1/user.cfc:102` method 'readByEmail' not found in userManager |
| 1 | `registerpublicentity` | 0 | none |  | `core/mura/client/api/feed/v1/feedApiUtility.cfc:58` not found in extends chain |
| 1 | `rethrow` | 0 | none |  | `core/mura/formBuilder/formBuilderManager.cfc:761` not found in extends chain |
| 1 | `setcategoryid` | 6 | low | declares setCategoryID; beside the calling file (6 candidates) | `core/mura/extend/extendObjectFeedBean.cfc:80` not found in extends chain |
| 1 | `setgroupid` | 3 | low | declares setGroupID (3 candidates) | `core/mura/extend/extendObjectFeedBean.cfc:84` not found in extends chain |
| 1 | `setinactive` | 2 | low | declares setInActive (2 candidates) | `core/mura/extend/extendObjectFeedBean.cfc:68` not found in extends chain |
| 1 | `setispublic` | 4 | low | declares setIsPublic (4 candidates) | `core/mura/extend/extendObjectFeedBean.cfc:72` not found in extends chain |
| 1 | `settranslator` | 0 | none |  | `core/mura/trash/trashIterator.cfc:86` method 'setTranslator' not found in trashItemBean |
| 1 | `siteconfig` | 1 | medium | declares siteConfig | `core/modules/v1/related_section_content/index.cfm:127` no qualifier, not in file |
| 1 | `valueexist` | 0 | none |  | `core/mura/bean/beanORM.cfc:604` not found in extends chain |
| 1 | `valueslist` | 0 | none |  | `core/mura/publisher.cfc:1559` not found in extends chain |

<details><summary>Groups with several candidates</summary>

- `parsexml` — 15 finding(s), 2 candidate(s):
  - low `core/mura/backport/acf.cfm:6` — declares parseXML
  - low `core/mura/backport/lucee.cfm:6` — declares parseXML
- `getsite` — 10 finding(s), 34 candidate(s):
  - low `core/mura/event.cfc:182` — declares getSite
  - low `core/mura/servletEvent.cfc:219` — declares getSite
  - low `core/mura/bean/beanORM.cfc:267` — declares getSite
  - low `core/mura/bean/beanORMVersioned.cfc:80` — declares getSite
  - low `core/mura/bean/beanRemotePointer.cfc:3` — declares getSite
  - low `core/mura/category/categoryBean.cfc:84` — declares getSite
  - low `core/mura/content/contentBean.cfc:84` — declares getSite
  - low `core/mura/content/contentCategoryAssignBean.cfc:84` — declares getSite
  - … 26 more
- `geturlstem` — 5 finding(s), 2 candidate(s):
  - low `core/mura/content/contentRenderer.cfc:2495` — declares getURLStem
  - low `core/mura/content/contentServer.cfc:946` — declares getURLStem
- `dspobject` — 4 finding(s), 2 candidate(s):
  - low `core/mura/content/contentRenderer.cfc:2388` — declares dspObject
  - low `core/mura/content/contentRendererUtility.cfc:1024` — declares dspObject
- `getpersonalizationid` — 4 finding(s), 2 candidate(s):
  - low `core/mura/content/contentRenderer.cfc:2440` — declares getPersonalizationID
  - low `core/mura/content/contentRendererUtility.cfc:470` — declares getPersonalizationID
- `getbean` — 2 finding(s), 20 candidate(s):
  - low `core/mura/MasaScope.cfc:389` — declares getBean
  - low `core/mura/cfobject.cfc:138` — declares getBean
  - low `core/mura/event.cfc:203` — declares getBean
  - low `core/mura/servletEvent.cfc:227` — declares getBean
  - low `core/mura/bean/ioc.cfc:199` — declares getBean
  - low `core/mura/category/categoryFeedBean.cfc:80` — declares getBean
  - low `core/mura/content/contentManager.cfc:142` — declares getBean
  - low `core/mura/email/emailManager.cfc:108` — declares getBean
  - … 12 more
- `getcurrenturl` — 2 finding(s), 2 candidate(s):
  - low `core/mura/content/contentRenderer.cfc:2672` — declares getCurrentURL
  - low `core/mura/content/contentRendererUtility.cfc:424` — declares getCurrentURL
- `getservicefactory` — 2 finding(s), 2 candidate(s):
  - low `core/mura/cfobject.cfc:134` — declares getServiceFactory
  - low `core/mura/event.cfc:190` — declares getServiceFactory
- `createcssid` — 1 finding(s), 2 candidate(s):
  - low `core/mura/content/contentRenderer.cfc:2602` — declares createCSSID
  - low `core/mura/content/contentRendererUtility.cfc:810` — declares createCSSID
- `createhref` — 1 finding(s), 4 candidate(s):
  - low `core/mura/MasaScope.cfc:412` — declares createHREF
  - low `core/mura/content/contentRenderer.cfc:2504` — declares createHREF
  - low `core/mura/content/contentRendererUtility.cfc:1563` — declares createHREF
  - low `core/mura/content/staticContentRenderer.cfc:81` — declares createHREF
- `generateeditablehook` — 1 finding(s), 2 candidate(s):
  - low `core/mura/content/contentRenderer.cfc:2997` — declares generateEditableHook
  - low `core/mura/content/contentRendererUtility.cfc:357` — declares generateEditableHook
- `getallvalues` — 1 finding(s), 25 candidate(s):
  - low `core/mura/configBean.cfc:1722` — declares getAllValues; beside the calling file
  - low `core/mura/event.cfc:141` — declares getAllValues; beside the calling file
  - low `core/mura/queryParam.cfc:210` — declares getAllValues; beside the calling file
  - low `core/mura/servletEvent.cfc:145` — declares getAllValues; beside the calling file
  - low `core/mura/bean/bean.cfc:471` — declares getAllValues
  - low `core/mura/bean/beanExtendable.cfc:284` — declares getAllValues
  - low `core/mura/content/contentBean.cfc:528` — declares getAllValues
  - low `core/mura/content/contentCommentBean.cfc:247` — declares getAllValues
  - … 17 more
- `getdbtype` — 1 finding(s), 3 candidate(s):
  - low `core/mura/bean/beanFeed.cfc:395` — declares getDbType; beside the calling file
  - low `core/mura/bean/beanORM.cfc:220` — declares getDbType; beside the calling file
  - low `core/mura/configBean.cfc:665` — declares getDbType
- `getsessionsearch` — 1 finding(s), 2 candidate(s):
  - low `core/mura/dashboard/dashboardManager.cfc:166` — declares getSessionSearch; beside the calling file
  - low `core/mura/user/sessionTracking/sessionTrackingGateway.cfc:610` — declares getSessionSearch
- `logerror` — 1 finding(s), 2 candidate(s):
  - low `core/mura/backport/acf.cfm:2` — declares logError
  - low `core/mura/backport/lucee.cfm:2` — declares logError
- `newresultquery` — 1 finding(s), 2 candidate(s):
  - low `core/mura/permission.cfc:756` — declares newResultQuery
  - low `core/mura/content/contentRenderer.cfc:2592` — declares newResultQuery
- `pathformat` — 1 finding(s), 2 candidate(s):
  - low `core/mura/Zip.cfc:764` — declares PathFormat; beside the calling file
  - low `core/mura/fileWriter.cfc:454` — declares PathFormat; beside the calling file
- `readaddress` — 1 finding(s), 3 candidate(s):
  - low `core/mura/user/userBean.cfc:663` — declares readAddress
  - low `core/mura/user/userDAO.cfc:720` — declares readAddress
  - low `core/mura/user/userManager.cfc:1138` — declares readAddress
- `setcategoryid` — 1 finding(s), 6 candidate(s):
  - low `core/mura/extend/extendSet.cfc:217` — declares setCategoryID; beside the calling file
  - low `core/mura/category/categoryBean.cfc:80` — declares setCategoryID
  - low `core/mura/user/userBean.cfc:356` — declares setCategoryID
  - low `core/mura/user/userFeedBean.cfc:224` — declares setCategoryID
  - low `core/mura/content/changeset/changesetBean.cfc:163` — declares setCategoryID
  - low `core/mura/content/feed/feedBean.cfc:281` — declares setCategoryID
- `setgroupid` — 1 finding(s), 3 candidate(s):
  - low `core/mura/email/emailBean.cfc:99` — declares setGroupID
  - low `core/mura/user/userBean.cfc:310` — declares setGroupID
  - low `core/mura/user/userFeedBean.cfc:208` — declares setGroupID
- `setinactive` — 1 finding(s), 2 candidate(s):
  - low `core/mura/user/userBean.cfc:101` — declares setInActive
  - low `core/mura/user/userFeedBean.cfc:185` — declares setInActive
- `setispublic` — 1 finding(s), 4 candidate(s):
  - low `core/mura/mailinglist/mailinglistBean.cfc:83` — declares setIsPublic
  - low `core/mura/user/userBean.cfc:102` — declares setIsPublic
  - low `core/mura/user/userFeedBean.cfc:192` — declares setIsPublic
  - low `core/mura/content/feed/feedBean.cfc:91` — declares setIsPublic

</details>

## Object references — a component path that names no file

| Findings | Group | Defined | Confidence | Evidence | Example |
|---:|---|---:|---|---|---|
| 8 | `component 'testWidget' does not exist (calling 'getErrors')` | 4 | none |  | `core/tests/specs/mura/core/entities.cfc:92` component 'testWidget' does not exist (calling 'getErrors') |
| 5 | `component 'javaLoader' does not exist (calling 'create')` | 18 | none |  | `core/mura/diffMatchPatch.cfc:6` component 'javaLoader' does not exist (calling 'create') |
| 4 | `chained on 'getSite', which is not found (calling 'getSite')` | 34 | none |  | `core/modules/v1/editprofile/index.cfm:362` chained on 'getSite', which is not found (calling 'getSite') |
| 4 | `component 'testWidget' does not exist (calling 'validate')` | 20 | none |  | `core/tests/specs/mura/core/entities.cfc:77` component 'testWidget' does not exist (calling 'validate') |
| 3 | `component 'marketingManager' does not exist (calling 'getDefaults')` | 0 | none |  | `core/mura/content/contentGatewayAdobe.cfc:441` component 'marketingManager' does not exist (calling 'getDefaults') |
| 3 | `component 'testWidget' does not exist (chain hop 'validate' to 'hasErrors')` | 1 | none |  | `core/tests/specs/mura/core/entities.cfc:260` component 'testWidget' does not exist (chain hop 'validate' to 'hasErrors') |
| 2 | `chained on 'getSite', which is not found (calling 'getExtranetPublicRegNotify')` | 1 | none |  | `core/modules/v1/editprofile/index.cfm:360` chained on 'getSite', which is not found (calling 'getExtranetPublicRegNotify') |
| 2 | `chained on 'getSite', which is not found (calling 'getTemplateIncludeDir')` | 1 | none |  | `core/modules/v1/component/index.cfm:123` chained on 'getSite', which is not found (calling 'getTemplateIncludeDir') |
| 2 | `component 'marketingManager' does not exist (calling 'getTrackingid')` | 0 | none |  | `core/mura/client/api/resource/variation.js.cfm:206` component 'marketingManager' does not exist (calling 'getTrackingid') |
| 2 | `component 'siteManager' does not exist (calling 'getSite')` | 34 | none |  | `core/mura/settings/settingsBundle.cfc:1680` component 'siteManager' does not exist (calling 'getSite') |
| 2 | `component 'siteManager' does not exist (chain hop 'getSite' to 'getFilePoolID')` | 1 | none |  | `core/mura/settings/settingsBundle.cfc:1680` component 'siteManager' does not exist (chain hop 'getSite' to 'getFilePoolID') |
| 2 | `component 'testWidget' does not exist (calling 'getOptionQuery')` | 0 | none |  | `core/tests/specs/mura/core/entities.cfc:304` component 'testWidget' does not exist (calling 'getOptionQuery') |
| 2 | `component 'testWidget' does not exist (calling 'setDateVar')` | 1 | none |  | `core/tests/specs/mura/core/entities.cfc:244` component 'testWidget' does not exist (calling 'setDateVar') |
| 2 | `component 'testWidget' does not exist (calling 'setDoubleVar')` | 1 | none |  | `core/tests/specs/mura/core/entities.cfc:220` component 'testWidget' does not exist (calling 'setDoubleVar') |
| 2 | `component 'testWidget' does not exist (calling 'setIntVar')` | 1 | none |  | `core/tests/specs/mura/core/entities.cfc:148` component 'testWidget' does not exist (calling 'setIntVar') |
| 2 | `component 'testWidget' does not exist (calling 'setSmallIntVar')` | 1 | none |  | `core/tests/specs/mura/core/entities.cfc:172` component 'testWidget' does not exist (calling 'setSmallIntVar') |
| 2 | `component 'testWidget' does not exist (calling 'setfloatVar')` | 1 | none |  | `core/tests/specs/mura/core/entities.cfc:196` component 'testWidget' does not exist (calling 'setfloatVar') |
| 2 | `component 'testWidget' does not exist (chain hop 'setDateVar' to 'getErrors')` | 4 | none |  | `core/tests/specs/mura/core/entities.cfc:244` component 'testWidget' does not exist (chain hop 'setDateVar' to 'getErrors') |
| 2 | `component 'testWidget' does not exist (chain hop 'setDateVar' to 'validate')` | 20 | none |  | `core/tests/specs/mura/core/entities.cfc:244` component 'testWidget' does not exist (chain hop 'setDateVar' to 'validate') |
| 2 | `component 'testWidget' does not exist (chain hop 'setDoubleVar' to 'getErrors')` | 4 | none |  | `core/tests/specs/mura/core/entities.cfc:220` component 'testWidget' does not exist (chain hop 'setDoubleVar' to 'getErrors') |
| 2 | `component 'testWidget' does not exist (chain hop 'setDoubleVar' to 'validate')` | 20 | none |  | `core/tests/specs/mura/core/entities.cfc:220` component 'testWidget' does not exist (chain hop 'setDoubleVar' to 'validate') |
| 2 | `component 'testWidget' does not exist (chain hop 'setIntVar' to 'getErrors')` | 4 | none |  | `core/tests/specs/mura/core/entities.cfc:148` component 'testWidget' does not exist (chain hop 'setIntVar' to 'getErrors') |
| 2 | `component 'testWidget' does not exist (chain hop 'setIntVar' to 'validate')` | 20 | none |  | `core/tests/specs/mura/core/entities.cfc:148` component 'testWidget' does not exist (chain hop 'setIntVar' to 'validate') |
| 2 | `component 'testWidget' does not exist (chain hop 'setSmallIntVar' to 'getErrors')` | 4 | none |  | `core/tests/specs/mura/core/entities.cfc:172` component 'testWidget' does not exist (chain hop 'setSmallIntVar' to 'getErrors') |
| 2 | `component 'testWidget' does not exist (chain hop 'setSmallIntVar' to 'validate')` | 20 | none |  | `core/tests/specs/mura/core/entities.cfc:172` component 'testWidget' does not exist (chain hop 'setSmallIntVar' to 'validate') |
| 2 | `component 'testWidget' does not exist (chain hop 'setfloatVar' to 'getErrors')` | 4 | none |  | `core/tests/specs/mura/core/entities.cfc:196` component 'testWidget' does not exist (chain hop 'setfloatVar' to 'getErrors') |
| 2 | `component 'testWidget' does not exist (chain hop 'setfloatVar' to 'validate')` | 20 | none |  | `core/tests/specs/mura/core/entities.cfc:196` component 'testWidget' does not exist (chain hop 'setfloatVar' to 'validate') |
| 1 | `chained on 'getCurrentUser', which is not found (calling 'isSuperUser')` | 1 | none |  | `core/modules/v1/gotofirstchild/index.cfm:86` chained on 'getCurrentUser', which is not found (calling 'isSuperUser') |
| 1 | `chained on 'getFieldService', which is not found (calling 'getBeanByAttributes')` | 0 | none |  | `core/mura/formBuilder/fieldBean.cfc:443` chained on 'getFieldService', which is not found (calling 'getBeanByAttributes') |
| 1 | `chained on 'getFieldService', which is not found (calling 'getFieldTypeService')` | 0 | none |  | `core/mura/formBuilder/fieldBean.cfc:443` chained on 'getFieldService', which is not found (calling 'getFieldTypeService') |
| 1 | `chained on 'getFormService', which is not found (calling 'getFieldService')` | 0 | none |  | `core/mura/formBuilder/formBean.cfc:262` chained on 'getFormService', which is not found (calling 'getFieldService') |
| 1 | `chained on 'getFormService', which is not found (calling 'getFormAttributes')` | 1 | none |  | `core/mura/formBuilder/formBean.cfc:249` chained on 'getFormService', which is not found (calling 'getFormAttributes') |
| 1 | `chained on 'getFormService', which is not found (calling 'getFormAttributeservice')` | 0 | none |  | `core/mura/formBuilder/formBean.cfc:249` chained on 'getFormService', which is not found (calling 'getFormAttributeservice') |
| 1 | `chained on 'getServiceFactory', which is not found (calling 'containsBean')` | 4 | none |  | `core/mura/client/api/resource/variation.js.cfm:196` chained on 'getServiceFactory', which is not found (calling 'containsBean') |
| 1 | `chained on 'getServiceFactory', which is not found (calling 'declareBean')` | 3 | none |  | `core/appcfc/onApplicationStart_include.cfm:486` chained on 'getServiceFactory', which is not found (calling 'declareBean') |
| 1 | `chained on 'getSite', which is not found (calling 'getExtranet')` | 1 | none |  | `core/modules/v1/search/index.cfm:108` chained on 'getSite', which is not found (calling 'getExtranet') |
| 1 | `chained on 'getSite', which is not found (calling 'getTemplateIncludePath')` | 1 | none |  | `core/modules/v1/component/index.cfm:125` chained on 'getSite', which is not found (calling 'getTemplateIncludePath') |
| 1 | `component 'fb2Utility' does not exist (calling 'queryToArray')` | 1 | none |  | `core/mura/formBuilder/formBuilderManager.cfc:844` component 'fb2Utility' does not exist (calling 'queryToArray') |
| 1 | `component 'geoCoding' does not exist (calling 'geocode')` | 1 | none |  | `core/mura/user/addressBean.cfc:203` component 'geoCoding' does not exist (calling 'geocode') |
| 1 | `component 'loginProvider' does not exist (calling 'validateResult')` | 5 | none |  | `core/modules/v1/login/model/handlers/authHandler.cfc:39` component 'loginProvider' does not exist (calling 'validateResult') |
| 1 | `component 'testOption' does not exist (calling 'checkSchema')` | 2 | none |  | `core/tests/specs/mura/core/entities.cfc:263` component 'testOption' does not exist (calling 'checkSchema') |
| 1 | `component 'testWidget' does not exist (calling 'addOption')` | 1 | none |  | `core/tests/specs/mura/core/entities.cfc:265` component 'testWidget' does not exist (calling 'addOption') |
| 1 | `component 'testWidget' does not exist (calling 'checkSchema')` | 2 | none |  | `core/tests/specs/mura/core/entities.cfc:76` component 'testWidget' does not exist (calling 'checkSchema') |
| 1 | `component 'testWidget' does not exist (calling 'exists')` | 4 | none |  | `core/tests/specs/mura/core/entities.cfc:295` component 'testWidget' does not exist (calling 'exists') |
| 1 | `component 'testWidget' does not exist (calling 'getOptionIterator')` | 0 | none |  | `core/tests/specs/mura/core/entities.cfc:312` component 'testWidget' does not exist (calling 'getOptionIterator') |
| 1 | `component 'testWidget' does not exist (calling 'hasErrors')` | 1 | none |  | `core/tests/specs/mura/core/entities.cfc:84` component 'testWidget' does not exist (calling 'hasErrors') |
| 1 | `component 'testWidget' does not exist (calling 'save')` | 45 | none |  | `core/tests/specs/mura/core/entities.cfc:288` component 'testWidget' does not exist (calling 'save') |
| 1 | `component 'testWidget' does not exist (calling 'setDescription')` | 9 | none |  | `core/tests/specs/mura/core/entities.cfc:132` component 'testWidget' does not exist (calling 'setDescription') |
| 1 | `component 'testWidget' does not exist (calling 'setEmail')` | 6 | none |  | `core/tests/specs/mura/core/entities.cfc:116` component 'testWidget' does not exist (calling 'setEmail') |
| 1 | `component 'testWidget' does not exist (calling 'setName')` | 21 | none |  | `core/tests/specs/mura/core/entities.cfc:100` component 'testWidget' does not exist (calling 'setName') |
| 1 | `component 'testWidget' does not exist (chain hop 'getOptionIterator' to 'getWidget')` | 1 | none |  | `core/tests/specs/mura/core/entities.cfc:312` component 'testWidget' does not exist (chain hop 'getOptionIterator' to 'getWidget') |
| 1 | `component 'testWidget' does not exist (chain hop 'getOptionIterator' to 'next')` | 1 | none |  | `core/tests/specs/mura/core/entities.cfc:312` component 'testWidget' does not exist (chain hop 'getOptionIterator' to 'next') |
| 1 | `component 'testWidget' does not exist (chain hop 'setDescription' to 'getErrors')` | 4 | none |  | `core/tests/specs/mura/core/entities.cfc:132` component 'testWidget' does not exist (chain hop 'setDescription' to 'getErrors') |
| 1 | `component 'testWidget' does not exist (chain hop 'setDescription' to 'validate')` | 20 | none |  | `core/tests/specs/mura/core/entities.cfc:132` component 'testWidget' does not exist (chain hop 'setDescription' to 'validate') |
| 1 | `component 'testWidget' does not exist (chain hop 'setEmail' to 'getErrors')` | 4 | none |  | `core/tests/specs/mura/core/entities.cfc:116` component 'testWidget' does not exist (chain hop 'setEmail' to 'getErrors') |
| 1 | `component 'testWidget' does not exist (chain hop 'setEmail' to 'validate')` | 20 | none |  | `core/tests/specs/mura/core/entities.cfc:116` component 'testWidget' does not exist (chain hop 'setEmail' to 'validate') |
| 1 | `component 'testWidget' does not exist (chain hop 'setName' to 'getErrors')` | 4 | none |  | `core/tests/specs/mura/core/entities.cfc:100` component 'testWidget' does not exist (chain hop 'setName' to 'getErrors') |
| 1 | `component 'testWidget' does not exist (chain hop 'setName' to 'validate')` | 20 | none |  | `core/tests/specs/mura/core/entities.cfc:100` component 'testWidget' does not exist (chain hop 'setName' to 'validate') |
