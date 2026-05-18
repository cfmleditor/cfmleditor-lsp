component {

    function init() {
        variables.widgets = [];
        return this;
    }

    function createWidget(required string name, string color = "red") {
        var widget = new models.Widget(arguments.name, arguments.color);
        return widget;
    }

    function renderAll() {
        var output = "";
        for (var w in variables.widgets) {
            output &= w.render();
        }
        return output;
    }

}
