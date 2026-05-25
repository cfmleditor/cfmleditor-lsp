component {
    public query function GetData(required string id) {
        return queryExecute("SELECT * FROM data WHERE id = :id", {id: arguments.id});
    }
    public query function GetReport(required string id) {
        return queryExecute("SELECT * FROM reports WHERE id = :id", {id: arguments.id});
    }
}
