angular.module('scionApp')
    .factory('registerService', ["$http", "$q", function($http, $q) {
    var registerService = {
        // Register a user
        register: function(registration) {
            // $http returns a promise, which has a then function, which also returns a promise
            return $http.post('/api/register', registration).then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        }
    };
    return registerService;
}]);
