component {
    public query function FetchRecord(required string id) {
        return queryExecute("SELECT * FROM records WHERE id = :id", {id: arguments.id});
    }

    public struct function FetchSummary(required string id) {
        return queryExecute("SELECT count(*) as total FROM records WHERE id = :id", {id: arguments.id});
    }

    public void function ClearCache() {
        // clears internal cache
    }
}
