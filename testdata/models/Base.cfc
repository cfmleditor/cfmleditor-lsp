component {

    function init() {
        return this;
    }

    function getClassName() {
        return getMetadata(this).name;
    }

}
