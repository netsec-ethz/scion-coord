scionApp
    .factory('registerService', ["$http", "$q", function ($http, $q) {
    return {
        // Get ReCaptcha site key
        getSiteKey: function (){
           return $http.get('/api/captchaSiteKey');
        },
        // Register a user
        register: function (registration) {
            return $http.post('/api/register', registration).then(function (response) {
                console.log(response);
                return response.data;
            });
        }
    };
}]);
