component {
    variables.persist = new persist();

    public query function GetData(required string id) {
        return VARIABLES.persist.FetchRecord(id=ARGUMENTS.id);
    }

    public struct function GetSummary(required string id) {
        return VARIABLES.persist.FetchSummary(id=ARGUMENTS.id);
    }

    public void function PurgeCache() {
        VARIABLES.persist.ClearCache();
    }
}
