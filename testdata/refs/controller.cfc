component {
    public query function GetData(required string id) {
        return VARIABLES.service.GetData(argumentCollection=ARGUMENTS);
    }
    public struct function GetReport() {
        var data = GetData(id="123");
        return {data: data};
    }
    public struct function RunReport(required string id) {
        return GetReport(id=ARGUMENTS.id);
    }
}
