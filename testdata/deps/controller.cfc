component {
    variables.service = new service();

    public struct function BuildReport(required string id) {
        var data = VARIABLES.service.GetData(id=ARGUMENTS.id);
        var summary = VARIABLES.service.GetSummary(id=ARGUMENTS.id);
        return {data: data, summary: summary};
    }

    public void function Cleanup() {
        VARIABLES.service.PurgeCache();
    }
}
