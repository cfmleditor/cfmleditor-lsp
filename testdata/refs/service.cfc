component {
    variables.persist = new persist();

    public query function GetData(required string id) {
        return VARIABLES.persist.GetData(argumentCollection=ARGUMENTS);
    }
    public query function GetReport(required string id) {
        return VARIABLES.persist.GetReport(argumentCollection=ARGUMENTS);
    }
    public void function OtherFunc() {
        var x = 1;
    }
}
