component extends="models.Base" {

    property name="name" type="string";
    property name="color" type="string";

    function init(required string name, string color = "blue") {
        variables.name = arguments.name;
        variables.color = arguments.color;
        return this;
    }

    function render() {
        return "<div class=""widget #variables.color#"">#variables.name#</div>";
    }

    function getName() {
        return variables.name;
    }

    function setColor(required string color) {
        variables.color = arguments.color;
    }

}
